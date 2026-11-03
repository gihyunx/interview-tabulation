package export

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gihyunx/interview-tabulation/internal/apitest"
	"github.com/gihyunx/interview-tabulation/internal/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

var (
	srv *httptest.Server
	fx  dbtest.Fixtures
)

func TestMain(m *testing.M) {
	db := dbtest.New(context.Background())
	fx = dbtest.Fixtures{Pool: db.Pool}
	srv = apitest.New(func(api huma.API) { Setup(api, db.Pool, apitest.Config) })
	code := m.Run()
	srv.Close()
	db.Close()
	os.Exit(code)
}

func TestWorkbook(t *testing.T) {
	rid, crits := fx.Rubric(t, 60, 40)
	one := fx.Interview(t, rid, "one_on_one")
	panel := fx.Interview(t, rid, "panel")
	oneJudge := fx.Judge(t, fx.User(t, "judge"), one)
	p1, p2 := fx.Judge(t, fx.User(t, "judge"), panel), fx.Judge(t, fx.User(t, "judge"), panel)
	a, b, unscored := fx.Candidate(t), fx.Candidate(t), fx.Candidate(t)

	for _, j := range []string{oneJudge} {
		fx.Score(t, j, a, crits[0], 50)
		fx.Score(t, j, a, crits[1], 30)
	}
	for _, j := range []string{p1, p2} {
		fx.Score(t, j, a, crits[0], 60)
		fx.Score(t, j, a, crits[1], 40)
	}
	fx.Score(t, p1, b, crits[0], 10)
	fx.Score(t, p1, b, crits[1], 10)

	resp := apitest.Do(t, srv, "GET", "/v1/results.xlsx?interview="+one+"&interview="+panel, apitest.AdminToken(t), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, xlsxType, resp.Header.Get("Content-Type"))
	assert.Regexp(t, regexp.MustCompile(`^attachment; filename="results-\d{4}-\d{2}-\d{2}-\d{4}\.xlsx"$`), resp.Header.Get("Content-Disposition"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer f.Close()

	var titles []string
	require.NoError(t, fx.Pool.QueryRow(context.Background(),
		`SELECT array_agg(title ORDER BY position, created_at) FROM interview WHERE interview_id IN ($1, $2)`, one, panel).Scan(&titles))
	sheets := f.GetSheetList()
	assert.Equal(t, append([]string{"Summary"}, titles...), sheets)

	t.Run("summary", func(t *testing.T) {
		rows, err := f.GetRows("Summary")
		require.NoError(t, err)
		head := rows[0]
		assert.Equal(t, "Candidate", head[0])
		assert.Contains(t, head[1], "Interviewer")
		assert.Equal(t, []string{"Sum", "Average (/100)", "Status"}, head[len(head)-3:])

		var aRow, bRow []string
		names := 0
		for _, r := range rows[1:] {
			names++
			if r[0] == lastName(t, a)+", Can" {
				aRow = r
			}
			if r[0] == lastName(t, b)+", Can" {
				bRow = r
			}
		}
		assert.Equal(t, 3, names, "every candidate, scored or not")
		require.NotNil(t, aRow)
		assert.Equal(t, "280", aRow[len(aRow)-3], "sum 80+100+100")
		assert.Equal(t, "93.33", aRow[len(aRow)-2][:5], "average")
		assert.Equal(t, "Complete", aRow[len(aRow)-1])
		assert.Equal(t, "Pending", bRow[len(bRow)-1])
	})

	t.Run("numbers are numbers", func(t *testing.T) {
		last := len(mustRows(t, f, "Summary")[0])
		sumRef, _ := excelize.CoordinatesToCellName(last-2, 2)
		typ, err := f.GetCellType("Summary", sumRef)
		require.NoError(t, err)
		assert.NotContains(t, []excelize.CellType{excelize.CellTypeSharedString, excelize.CellTypeInlineString}, typ, "stored as a number, not text")
	})

	t.Run("detail sheet", func(t *testing.T) {
		rows := mustRows(t, f, titles[1])
		assert.Equal(t, "Candidate", rows[0][0])
		assert.Equal(t, "Judge", rows[0][1])
		assert.Regexp(t, `\(/60\)$`, rows[0][2])
		assert.Equal(t, []string{"Total", "Submitted at", "Last edited at"}, rows[0][4:7])
		var unscoredRow []string
		for _, r := range rows[1:] {
			if r[0] == lastName(t, unscored)+", Can" {
				unscoredRow = r
			}
		}
		assert.Equal(t, []string{lastName(t, unscored) + ", Can"}, unscoredRow, "blank row for an unscored candidate")
		assert.Len(t, rows[1:], 4, "a x 2 judges, b x 1 judge, unscored")
	})
}

func mustRows(t *testing.T, f *excelize.File, sheet string) [][]string {
	rows, err := f.GetRows(sheet)
	require.NoError(t, err)
	return rows
}

func lastName(t *testing.T, id string) (n string) {
	require.NoError(t, fx.Pool.QueryRow(context.Background(), `SELECT last_name FROM candidate WHERE candidate_id = $1`, id).Scan(&n))
	return
}

func TestSheetNames(t *testing.T) {
	used := map[string]bool{}
	a := uniqueSheetName("Panel: Final/Round [1]?", used)
	assert.Equal(t, "Panel- Final-Round -1--", a)
	b := uniqueSheetName("panel- final-round -1--", used)
	assert.NotEqual(t, a, b)
	long := uniqueSheetName("A very long interview title that goes past thirty one", used)
	assert.LessOrEqual(t, len([]rune(long)), 31)
	assert.NotEqual(t, "Summary", uniqueSheetName("Summary", used))
}

func TestAdminOnly(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, apitest.Do(t, srv, "GET", "/v1/results.xlsx", apitest.JudgeToken(t, "u"), nil).StatusCode)
	assert.Equal(t, http.StatusUnauthorized, apitest.Do(t, srv, "GET", "/v1/results.xlsx", "", nil).StatusCode)
}
