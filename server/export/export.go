package export

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata" // the report is in Asia/Manila even where the OS has no zoneinfo
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/config"
	"github.com/gihyunx/interview-tabulation/internal/apierr"
	"github.com/gihyunx/interview-tabulation/internal/apitag"
	"github.com/gihyunx/interview-tabulation/internal/auth"
	"github.com/gihyunx/interview-tabulation/score"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

const (
	xlsxType   = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	timeLayout = "2006-01-02 15:04"
)

func Setup(api huma.API, pool *pgxpool.Pool, cfg config.Config) {
	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		panic(err)
	}

	huma.Register(api, huma.Operation{
		OperationID: "results-xlsx", Method: http.MethodGet, Path: "/results.xlsx",
		Tags: []string{apitag.Results}, Summary: "Download results as .xlsx",
		Description: "Same `interview` selection as /results.",
		Middlewares: huma.Middlewares{auth.RequireAuth(api, cfg.JWTSecret, auth.RoleAdmin)},
		Responses: map[string]*huma.Response{"200": {
			Description: "The workbook",
			Content:     map[string]*huma.MediaType{xlsxType: {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
		}},
	}, func(ctx context.Context, in *struct {
		Interview []string `query:"interview,explode" format:"uuid"`
	}) (*huma.StreamResponse, error) {
		res, err := score.ReadResults(ctx, pool, in.Interview)
		if err != nil {
			return nil, apierr.Huma(err)
		}
		now := time.Now().In(loc)
		f := build(res, loc)
		defer f.Close()

		// The file is built before the first byte is sent, so build errors are still a clean 500.
		buf, err := f.WriteToBuffer()
		if err != nil {
			return nil, err
		}
		return &huma.StreamResponse{Body: func(ctx huma.Context) {
			ctx.SetHeader("Content-Type", xlsxType)
			ctx.SetHeader("Content-Disposition", fmt.Sprintf(`attachment; filename="results-%s.xlsx"`, now.Format("2006-01-02-1504")))
			_, _ = buf.WriteTo(ctx.BodyWriter())
		}}, nil
	})
}

func build(res score.Results, loc *time.Location) *excelize.File {
	f := excelize.NewFile()
	// Formulas are saved without results; make Excel and LibreOffice compute them on open.
	_ = f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: new(true)})
	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})

	var selected []score.ResultInterview
	for _, i := range res.Interviews {
		if i.Selected {
			selected = append(selected, i)
		}
	}

	summary(f, bold, res, selected)
	used := map[string]bool{}
	for _, in := range selected {
		name := uniqueSheetName(in.Title, used)
		if _, err := f.NewSheet(name); err != nil {
			continue
		}
		detail(f, bold, name, res, in, loc)
	}
	return f
}

func summary(f *excelize.File, bold int, res score.Results, selected []score.ResultInterview) {
	const sheet = "Summary"
	threeDP := "0.000"
	avgStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &threeDP})
	_ = f.SetSheetName("Sheet1", sheet)

	head := []any{"No.", "Candidate"}
	for _, in := range selected {
		if in.Mode == "one_on_one" {
			head = append(head, in.Title+" - Interviewer", in.Title+" - Total")
			continue
		}
		for _, j := range in.Judges {
			head = append(head, in.Title+" - "+j.Name)
		}
	}
	head = append(head, "Sum", "Average (/100)", "Status")
	row(f, sheet, 1, head)

	for n, c := range res.Candidates {
		cells := []any{c.Number, c.LastName + ", " + c.FirstName}
		for _, in := range selected {
			if in.Mode == "one_on_one" {
				if s := submission(c, in.ID, ""); s != nil {
					cells = append(cells, s.JudgeName, s.Total)
				} else {
					cells = append(cells, nil, nil)
				}
				continue
			}
			for _, j := range in.Judges {
				if s := submission(c, in.ID, j.JudgeID); s != nil {
					cells = append(cells, s.Total)
				} else {
					cells = append(cells, nil)
				}
			}
		}
		r := n + 2
		row(f, sheet, r, cells)
		// Sum, average and status are formulas over the score columns, so they can be checked.
		// SUM and COUNT skip the interviewer names, leaving only the res.Expected totals.
		scores := ""
		if len(cells) > 2 {
			from, _ := excelize.CoordinatesToCellName(3, r)
			to, _ := excelize.CoordinatesToCellName(len(cells), r)
			scores = from + ":" + to
		}
		sumRef, _ := excelize.CoordinatesToCellName(len(cells)+1, r)
		if scores == "" || res.Expected == 0 {
			row(f, sheet, r, append(cells, 0, nil, "Pending"))
			continue
		}
		done := fmt.Sprintf("COUNT(%s)=%d", scores, res.Expected)
		formula(f, sheet, len(cells)+1, r, fmt.Sprintf("SUM(%s)", scores))
		formula(f, sheet, len(cells)+2, r, fmt.Sprintf(`IF(%s,%s/%d,"")`, done, sumRef, res.Expected))
		avgRef, _ := excelize.CoordinatesToCellName(len(cells)+2, r)
		_ = f.SetCellStyle(sheet, avgRef, avgRef, avgStyle)
		formula(f, sheet, len(cells)+3, r, fmt.Sprintf(`IF(%s,"Complete","Pending")`, done))
	}
	finish(f, sheet, bold, len(head))
}

func detail(f *excelize.File, bold int, sheet string, res score.Results, in score.ResultInterview, loc *time.Location) {
	head := []any{"No.", "Candidate", "Judge"}
	for _, c := range in.Criteria {
		head = append(head, fmt.Sprintf("%s (/%d)", c.Title, c.MaxPoints))
	}
	head = append(head, "Total", "Submitted at", "Last edited at")
	row(f, sheet, 1, head)

	r := 2
	for _, c := range res.Candidates {
		name := c.LastName + ", " + c.FirstName
		subs := 0
		for _, s := range c.Submissions {
			if s.InterviewID != in.ID {
				continue
			}
			subs++
			values := map[string]int16{}
			for _, v := range s.Values {
				values[v.CriterionID] = v.Value
			}
			cells := []any{c.Number, name, s.JudgeName}
			for _, cr := range in.Criteria {
				cells = append(cells, values[cr.ID])
			}
			total := len(cells) + 1
			row(f, sheet, r, append(cells, nil, s.SubmittedAt.In(loc).Format(timeLayout), s.EditedAt.In(loc).Format(timeLayout)))
			if len(in.Criteria) > 0 {
				from, _ := excelize.CoordinatesToCellName(4, r)
				to, _ := excelize.CoordinatesToCellName(total-1, r)
				formula(f, sheet, total, r, fmt.Sprintf("SUM(%s:%s)", from, to))
			}
			r++
		}
		if subs == 0 { // keep the candidate list complete
			row(f, sheet, r, []any{c.Number, name})
			r++
		}
	}
	finish(f, sheet, bold, len(head))
}

func submission(c score.ResultCandidate, interviewID, judgeID string) *score.Submission {
	for i := range c.Submissions {
		s := &c.Submissions[i]
		if s.InterviewID == interviewID && (judgeID == "" || s.JudgeID == judgeID) {
			return s
		}
	}
	return nil
}

func row(f *excelize.File, sheet string, n int, cells []any) {
	ref, _ := excelize.CoordinatesToCellName(1, n)
	_ = f.SetSheetRow(sheet, ref, &cells)
}

func formula(f *excelize.File, sheet string, col, n int, expr string) {
	ref, _ := excelize.CoordinatesToCellName(col, n)
	_ = f.SetCellFormula(sheet, ref, expr)
}

// finish makes the header bold and frozen and fits each column to its longest text, like
// double-clicking the column edge. Formula cells read as empty here; their numbers are short.
func finish(f *excelize.File, sheet string, bold, cols int) {
	last, _ := excelize.CoordinatesToCellName(cols, 1)
	_ = f.SetCellStyle(sheet, "A1", last, bold)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	widths := make([]int, cols)
	rows, _ := f.GetRows(sheet)
	for _, r := range rows {
		for c, v := range r[:min(len(r), cols)] {
			widths[c] = max(widths[c], utf8.RuneCountInString(v))
		}
	}
	for c, w := range widths {
		name, _ := excelize.ColumnNumberToName(c + 1)
		// ponytail: character count, not measured glyphs; +2 covers bold headers and padding.
		_ = f.SetColWidth(sheet, name, name, float64(max(w, 6)+2))
	}
}

// uniqueSheetName makes a title valid as a sheet name (31 chars, no : \ / ? * [ ]) and unused.
func uniqueSheetName(title string, used map[string]bool) string {
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:\/?*[]`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(title))
	if name == "" {
		name = "Interview"
	}
	base := []rune(name)
	if len(base) > 31 {
		base = base[:31]
	}
	name = string(base)
	for n := 2; used[strings.ToLower(name)] || strings.EqualFold(name, "Summary"); n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		name = string(base[:min(len(base), 31-len(suffix))]) + suffix
	}
	used[strings.ToLower(name)] = true
	return name
}
