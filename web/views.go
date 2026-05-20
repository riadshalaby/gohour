package web

import (
	"fmt"
	"time"

	"github.com/riadshalaby/gohour/config"
	"github.com/riadshalaby/gohour/importer"
	"github.com/riadshalaby/gohour/internal/timeutil"
	"github.com/riadshalaby/gohour/onepoint"
	"github.com/riadshalaby/gohour/worklog"
)

type monthRowView struct {
	Date               string  `json:"date"`
	IsWeekend          bool    `json:"isWeekend"`
	IsToday            bool    `json:"isToday"`
	HasLockedRemote    bool    `json:"hasLockedRemote"`
	LocalHours         float64 `json:"localHours"`
	RemoteHours        float64 `json:"remoteHours"`
	LocalWorked        float64 `json:"localWorked"`
	RemoteWorked       float64 `json:"remoteWorked"`
	WorkedDeltaHours   float64 `json:"workedDeltaHours"`
	BillableDeltaHours float64 `json:"billableDeltaHours"`
	DayLink            string  `json:"dayLink"`
}

type monthPageView struct {
	Title         string
	CurrentMonth  string
	PreviousMonth string
	NextMonth     string
	// Day is intentionally empty for month pages; defined here so the shared
	// base.html template can safely access .Day without causing a template error.
	Day                string
	AuthErrorMsg       string
	Rows               []monthRowView
	TotalLocal         float64
	TotalRemote        float64
	TotalLocalWorked   float64
	TotalRemoteWorked  float64
	TotalWorkedDelta   float64
	TotalBillableDelta float64
	RemoteRefreshedAt  string
}

type dayPageView struct {
	Title             string
	CurrentMonth      string
	Day               string
	AuthErrorMsg      string
	DayRow            DayRow
	RemoteRefreshedAt string
}

type dayAPIResponse struct {
	Date              string     `json:"date"`
	LocalHours        float64    `json:"localHours"`
	RemoteHours       float64    `json:"remoteHours"`
	LocalWorkedHours  float64    `json:"localWorkedHours"`
	RemoteWorkedHours float64    `json:"remoteWorkedHours"`
	Entries           []EntryRow `json:"entries"`
	RemoteRefreshedAt string     `json:"remoteRefreshedAt,omitempty"`
}

type configPageView struct {
	Title        string
	CurrentMonth string
	Day          string
	AuthErrorMsg string
	Config       config.Config
	Rules        []config.Rule
}

type configAPIResponse struct {
	OnePointURL string        `json:"onepointUrl"`
	Rules       []rulePayload `json:"rules"`
}

type configPatchRequest struct {
	OnePointURL *string `json:"onepointUrl"`
}

type rulePayload struct {
	Name         string `json:"name"`
	Mapper       string `json:"mapper"`
	FileTemplate string `json:"fileTemplate"`
	Billable     *bool  `json:"billable,omitempty"`
	ProjectID    int64  `json:"projectId"`
	Project      string `json:"project"`
	ActivityID   int64  `json:"activityId"`
	Activity     string `json:"activity"`
	SkillID      int64  `json:"skillId"`
	Skill        string `json:"skill"`
}

type monthAPIResponse struct {
	Month              string         `json:"month"`
	Rows               []monthRowView `json:"rows"`
	TotalLocal         float64        `json:"totalLocal"`
	TotalRemote        float64        `json:"totalRemote"`
	TotalLocalWorked   float64        `json:"totalLocalWorked"`
	TotalRemoteWorked  float64        `json:"totalRemoteWorked"`
	TotalWorkedDelta   float64        `json:"totalWorkedDelta"`
	TotalBillableDelta float64        `json:"totalBillableDelta"`
	AuthErrorMsg       string         `json:"authErrorMsg,omitempty"`
	RemoteRefreshedAt  string         `json:"remoteRefreshedAt,omitempty"`
}

type worklogMutationRequest struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Project     string `json:"project"`
	Activity    string `json:"activity"`
	Skill       string `json:"skill"`
	Billable    int    `json:"billable"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

type importResponse struct {
	FilesProcessed   int    `json:"filesProcessed"`
	RowsRead         int    `json:"rowsRead"`
	RowsMapped       int    `json:"rowsMapped"`
	RowsSkipped      int    `json:"rowsSkipped"`
	RowsPersisted    int    `json:"rowsPersisted"`
	ReconcileWarning string `json:"reconcileWarning,omitempty"`
	OverlapsSkipped  int    `json:"overlapsSkipped,omitempty"`
}

type importPreviewEntry struct {
	Index        int    `json:"index"`
	Date         string `json:"date"`
	Start        string `json:"start"`
	End          string `json:"end"`
	Project      string `json:"project"`
	Activity     string `json:"activity"`
	Skill        string `json:"skill"`
	BillableMins int    `json:"billableMins"`
	DurationMins int    `json:"durationMins"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	ConflictID   int64  `json:"conflictId,omitempty"`
}

type importPreviewResponse struct {
	RowsMapped  int                  `json:"rowsMapped"`
	RowsSkipped int                  `json:"rowsSkipped"`
	Entries     []importPreviewEntry `json:"entries"`
	MatchedRule *rulePayload         `json:"matchedRule,omitempty"`
	Selection   rulePayload          `json:"selection"`
	Mappers     []string             `json:"mappers"`
	Lookup      *lookupResponse      `json:"lookup,omitempty"`
}

type importRuleMatchResponse struct {
	MatchedRule *rulePayload    `json:"matchedRule,omitempty"`
	Selection   rulePayload     `json:"selection"`
	Mappers     []string        `json:"mappers"`
	Lookup      *lookupResponse `json:"lookup,omitempty"`
}

type importFormResult struct {
	tmpPath     string
	result      *importer.Result
	mapperName  string
	matchedRule config.Rule
	selection   rulePayload
	updateRule  bool
}

type importOverlapItem struct {
	Date       string `json:"date"`
	Start      string `json:"start"`
	End        string `json:"end"`
	Project    string `json:"project"`
	Activity   string `json:"activity"`
	Skill      string `json:"skill"`
	ExistingID int64  `json:"existingId"`
}

type importConflictResponse struct {
	Error      string              `json:"error"`
	Overlaps   []importOverlapItem `json:"overlaps"`
	CleanCount int                 `json:"cleanCount"`
	Duplicates int                 `json:"duplicates"`
}

type submitDayResult struct {
	Date       string `json:"date"`
	Added      int    `json:"added"`
	Duplicates int    `json:"duplicates"`
	Overlaps   int    `json:"overlaps"`
	Locked     bool   `json:"locked"`
}

type submitResponse struct {
	DryRun     bool              `json:"dryRun,omitempty"`
	Submitted  int               `json:"submitted"`
	Duplicates int               `json:"duplicates"`
	Overlaps   int               `json:"overlaps"`
	LockedDays []string          `json:"lockedDays"`
	Days       []submitDayResult `json:"days"`
}

type worklogConflictResponse struct {
	Error      string `json:"error"`
	Type       string `json:"type"`
	ExistingID int64  `json:"existingId"`
}

type submitPartialView struct {
	Scope   string
	Target  string
	DryRun  bool
	Result  submitResponse
	Error   string
	IsError bool
}

type lookupProject struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

type lookupActivity struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ProjectID int64  `json:"projectId"`
	Locked    bool   `json:"locked"`
}

type lookupSkill struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ActivityID int64  `json:"activityId"`
}

type lookupResponse struct {
	Projects   []lookupProject  `json:"projects"`
	Activities []lookupActivity `json:"activities"`
	Skills     []lookupSkill    `json:"skills"`
}

func buildMonthRows(monthStart time.Time, localEntries []worklog.Entry, remoteEntries []onepoint.DayWorklog) ([]monthRowView, MonthSummary) {
	dayRows := BuildDailyView(localEntries, remoteEntries)
	dayRows = fillMonthDays(monthStart, dayRows)
	summary := BuildMonthlyView(dayRows)
	lockedByDay := make(map[string]bool)
	for _, item := range remoteEntries {
		if item.Locked == 0 {
			continue
		}
		day, err := onepoint.ParseDay(item.WorklogDate)
		if err != nil {
			continue
		}
		lockedByDay[timeutil.StartOfDay(day).Format("2006-01-02")] = true
	}

	now := timeutil.StartOfDay(time.Now())
	rows := make([]monthRowView, 0, len(summary.Days))
	for _, day := range summary.Days {
		dayDate := timeutil.StartOfDay(day.Date)
		dayISO := dayDate.Format("2006-01-02")
		wd := dayDate.Weekday()
		rows = append(rows, monthRowView{
			Date:               dayISO,
			IsWeekend:          wd == time.Saturday || wd == time.Sunday,
			IsToday:            dayDate.Equal(now),
			HasLockedRemote:    lockedByDay[dayISO],
			LocalHours:         day.LocalHours,
			RemoteHours:        day.RemoteHours,
			LocalWorked:        day.LocalWorkedHours,
			RemoteWorked:       day.RemoteWorkedHours,
			WorkedDeltaHours:   day.LocalWorkedHours - day.RemoteWorkedHours,
			BillableDeltaHours: day.DeltaHours,
			DayLink:            "/day/" + dayISO,
		})
	}

	return rows, summary
}

func fillMonthDays(monthStart time.Time, rows []DayRow) []DayRow {
	index := make(map[string]DayRow, len(rows))
	for _, row := range rows {
		index[timeutil.StartOfDay(row.Date).Format("2006-01-02")] = row
	}

	monthEnd := endOfMonth(monthStart)
	out := make([]DayRow, 0, monthEnd.Day())
	for day := monthStart; !day.After(monthEnd); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if row, ok := index[key]; ok {
			out = append(out, row)
			continue
		}
		out = append(out, DayRow{Date: day})
	}
	return out
}

func endOfMonth(monthStart time.Time) time.Time {
	return monthStart.AddDate(0, 1, -1)
}

func rangeDays(from, to time.Time) []time.Time {
	out := make([]time.Time, 0, 32)
	for day := timeutil.StartOfDay(from); !day.After(to); day = day.AddDate(0, 0, 1) {
		out = append(out, day)
	}
	return out
}

func lookupProjectName(snap onepoint.LookupSnapshot, id int64) string {
	for _, project := range snap.Projects {
		if project.ID == id {
			return project.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
}

func lookupActivityName(snap onepoint.LookupSnapshot, id int64) string {
	for _, activity := range snap.Activities {
		if activity.ID == id {
			return activity.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
}

func lookupSkillName(snap onepoint.LookupSnapshot, id int64) string {
	for _, skill := range snap.Skills {
		if skill.SkillID == id {
			return skill.Name
		}
	}
	return fmt.Sprintf("id:%d", id)
}

func lookupResponseFromSnapshot(snapshot onepoint.LookupSnapshot) lookupResponse {
	resp := lookupResponse{
		Projects:   make([]lookupProject, 0, len(snapshot.Projects)),
		Activities: make([]lookupActivity, 0, len(snapshot.Activities)),
		Skills:     make([]lookupSkill, 0, len(snapshot.Skills)),
	}
	for _, p := range snapshot.Projects {
		resp.Projects = append(resp.Projects, lookupProject{
			ID:       p.ID,
			Name:     p.Name,
			Archived: p.IsArchived(),
		})
	}
	for _, a := range snapshot.Activities {
		resp.Activities = append(resp.Activities, lookupActivity{
			ID:        a.ID,
			Name:      a.Name,
			ProjectID: a.ProjectNodeID,
			Locked:    a.Locked,
		})
	}
	for _, sk := range snapshot.Skills {
		resp.Skills = append(resp.Skills, lookupSkill{
			ID:         sk.SkillID,
			Name:       sk.Name,
			ActivityID: sk.ActivityID,
		})
	}
	return resp
}
