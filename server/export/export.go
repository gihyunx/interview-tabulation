package export

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	_ "time/tzdata" // the report is in Asia/Manila even where the OS has no zoneinfo

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
		avg, status := any(nil), "Pending"
		if c.Complete {
			avg, status = *c.Average, "Complete"
		}
		row(f, sheet, n+2, append(cells, c.Sum, avg, status))
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
			row(f, sheet, r, append(cells, s.Total, s.SubmittedAt.In(loc).Format(timeLayout), s.EditedAt.In(loc).Format(timeLayout)))
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

// finish makes the header bold and frozen and sizes the columns.
func finish(f *excelize.File, sheet string, bold, cols int) {
	last, _ := excelize.CoordinatesToCellName(cols, 1)
	_ = f.SetCellStyle(sheet, "A1", last, bold)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	for c := 1; c <= cols; c++ {
		name, _ := excelize.ColumnNumberToName(c)
		w := 16.0
		if c == 1 {
			w = 28
		}
		_ = f.SetColWidth(sheet, name, name, w)
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
