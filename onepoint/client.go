package onepoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	dayLayout         = "02-01-2006"
	defaultRefererURI = "/onepoint/faces/home"
)

// Client defines the OnePoint API operations known from discovery.
type Client interface {
	ListProjects(ctx context.Context) ([]Project, error)
	ListActivities(ctx context.Context) ([]Activity, error)
	ListSkills(ctx context.Context) ([]Skill, error)
	GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]DayWorklog, error)
	GetDayWorklogs(ctx context.Context, day time.Time) ([]DayWorklog, error)
	PersistWorklogs(ctx context.Context, day time.Time, worklogs []PersistWorklog) ([]PersistResult, error)
	FetchLookupSnapshot(ctx context.Context) (LookupSnapshot, error)
	ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options ResolveOptions) (ResolvedIDs, error)
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type ClientConfig struct {
	BaseURL        string
	RefererURL     string
	SessionCookies string
	UserAgent      string
	HTTPClient     httpDoer
}

type HTTPClient struct {
	baseURL        string
	refererURL     string
	sessionCookies string
	userAgent      string
	httpClient     httpDoer
}

func NewClient(cfg ClientConfig) (*HTTPClient, error) {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")

	parsedBase, err := url.Parse(baseURL)
	if err != nil || parsedBase.Scheme == "" || parsedBase.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q", cfg.BaseURL)
	}

	refererURL := strings.TrimSpace(cfg.RefererURL)
	if refererURL == "" {
		refererURL = baseURL + defaultRefererURI
	}

	doer := cfg.HTTPClient
	if doer == nil {
		doer = &http.Client{Timeout: 30 * time.Second}
	}

	return &HTTPClient{
		baseURL:        baseURL,
		refererURL:     refererURL,
		sessionCookies: strings.TrimSpace(cfg.SessionCookies),
		userAgent:      strings.TrimSpace(cfg.UserAgent),
		httpClient:     doer,
	}, nil
}

type Project struct {
	ID       int64  `json:"opId"`
	Name     string `json:"opName"`
	Archived string `json:"opArchived"`
	Status   int64  `json:"opStatus"`
}

func (p Project) IsArchived() bool {
	return strings.TrimSpace(p.Archived) == "1"
}

type Activity struct {
	ID              int64  `json:"activityId"`
	Locked          bool   `json:"locked"`
	Name            string `json:"name"`
	ProjectNodeID   int64  `json:"projectNodeId"`
	SuperActivityID int64  `json:"superActivityId"`
}

type Skill struct {
	ActivityID      int64  `json:"activityId"`
	Name            string `json:"name"`
	SkillID         int64  `json:"skillId"`
	SuperCategoryID int64  `json:"superCategoryId"`
}

type DayWorklog struct {
	ActivityID   int64  `json:"activityId"`
	Billable     int    `json:"billable"`
	Comment      string `json:"comment"`
	Duration     int    `json:"duration"`
	FinishTime   int    `json:"finishTime"`
	Locked       int    `json:"locked"`
	ProjectID    int64  `json:"projectId"`
	SkillID      int64  `json:"skillId"`
	StartTime    int    `json:"startTime"`
	TimeRecordID int64  `json:"timerecordId"`
	Valuable     int    `json:"valuable"`
	WorklogDate  string `json:"worklogDate"`
	WorkRecordID int64  `json:"workrecordId"`
	WorkSlipID   int64  `json:"workslipId"`
}

func (w DayWorklog) ToPersistWorklog() PersistWorklog {
	start := w.StartTime
	finish := w.FinishTime
	return PersistWorklog{
		TimeRecordID: w.TimeRecordID,
		WorkSlipID:   w.WorkSlipID,
		WorkRecordID: w.WorkRecordID,
		WorklogDate:  w.WorklogDate,
		StartTime:    &start,
		FinishTime:   &finish,
		Duration:     w.Duration,
		Billable:     w.Billable,
		Valuable:     w.Valuable,
		ProjectID:    ID(w.ProjectID),
		ActivityID:   ID(w.ActivityID),
		SkillID:      ID(w.SkillID),
		Comment:      w.Comment,
	}
}

type getFilteredWorklogsResponse struct {
	Worklogs []DayWorklog `json:"worklogs"`
}

// FlexibleInt64 supports payload fields that can be numeric IDs or empty strings.
type FlexibleInt64 struct {
	Valid bool
	Value int64
}

func ID(value int64) FlexibleInt64 {
	return FlexibleInt64{Valid: true, Value: value}
}

func (id FlexibleInt64) MarshalJSON() ([]byte, error) {
	if !id.Valid {
		return []byte(`""`), nil
	}
	return []byte(strconv.FormatInt(id.Value, 10)), nil
}

func (id *FlexibleInt64) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	switch text {
	case "", "null", `""`:
		*id = FlexibleInt64{}
		return nil
	}

	var number int64
	if err := json.Unmarshal(data, &number); err == nil {
		*id = FlexibleInt64{Valid: true, Value: number}
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			*id = FlexibleInt64{}
			return nil
		}
		parsed, err := strconv.ParseInt(asString, 10, 64)
		if err != nil {
			return fmt.Errorf("parse id string %q: %w", asString, err)
		}
		*id = FlexibleInt64{Valid: true, Value: parsed}
		return nil
	}

	return fmt.Errorf("unsupported id value %q", text)
}

type PersistWorklog struct {
	TimeRecordID int64         `json:"timerecordId"`
	WorkSlipID   int64         `json:"workslipId"`
	WorkRecordID int64         `json:"workrecordId"`
	WorklogDate  string        `json:"worklogDate"`
	StartTime    *int          `json:"startTime"`
	FinishTime   *int          `json:"finishTime"`
	Duration     int           `json:"duration"`
	Billable     int           `json:"billable"`
	Valuable     int           `json:"valuable"`
	ProjectID    FlexibleInt64 `json:"projectId"`
	ActivityID   FlexibleInt64 `json:"activityId"`
	SkillID      FlexibleInt64 `json:"skillId"`
	Comment      string        `json:"comment"`
}

type PersistResult struct {
	Message         string `json:"message"`
	MessageType     string `json:"messageType"`
	NewTimeRecordID int64  `json:"newTimeRecordId"`
	OldTimeRecordID int64  `json:"oldTimeRecordId"`
	WorkRecordID    int64  `json:"workRecordId"`
	WorkSlipID      int64  `json:"workSlipId"`
	WorklogDate     string `json:"worklogDate"`
}

type LookupSnapshot struct {
	Projects   []Project
	Activities []Activity
	Skills     []Skill
}

type ResolveOptions struct {
	IncludeArchivedProjects bool
	IncludeLockedActivities bool
}

type ResolvedIDs struct {
	ProjectID    int64
	ActivityID   int64
	SkillID      int64
	ProjectName  string
	ActivityName string
	SkillName    string
}

func (c *HTTPClient) ListProjects(ctx context.Context) ([]Project, error) {
	var out []Project
	if err := c.doJSON(ctx, http.MethodPost, "/OPServices/resources/OpProjects/getAllUserProjects?mode=all", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) ListActivities(ctx context.Context) ([]Activity, error) {
	var out []Activity
	if err := c.doJSON(ctx, http.MethodPost, "/OPServices/resources/OpProjects/getAllUserActivities?mode=all", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) ListSkills(ctx context.Context) ([]Skill, error) {
	var out []Skill
	if err := c.doJSON(ctx, http.MethodPost, "/OPServices/resources/OpProjects/getAllUserSkills?mode=all", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) GetFilteredWorklogs(ctx context.Context, from, to time.Time) ([]DayWorklog, error) {
	path := fmt.Sprintf(
		"/OPServices/resources/OpWorklogs/%s:%s/getFilteredWorklogs",
		FormatDay(from),
		FormatDay(to),
	)
	var out getFilteredWorklogsResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Worklogs, nil
}

func (c *HTTPClient) GetDayWorklogs(ctx context.Context, day time.Time) ([]DayWorklog, error) {
	return c.GetFilteredWorklogs(ctx, day, day)
}

func (c *HTTPClient) PersistWorklogs(ctx context.Context, day time.Time, worklogs []PersistWorklog) ([]PersistResult, error) {
	if len(worklogs) == 0 {
		return nil, errors.New("persist worklogs payload must not be empty")
	}

	path := fmt.Sprintf("/OPServices/resources/OpWorklogs/%s/persistWorklogs", FormatDay(day))
	var out []PersistResult
	if err := c.doJSON(ctx, http.MethodPost, path, worklogs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) FetchLookupSnapshot(ctx context.Context) (LookupSnapshot, error) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return LookupSnapshot{}, err
	}
	activities, err := c.ListActivities(ctx)
	if err != nil {
		return LookupSnapshot{}, err
	}
	skills, err := c.ListSkills(ctx)
	if err != nil {
		return LookupSnapshot{}, err
	}
	return LookupSnapshot{
		Projects:   projects,
		Activities: activities,
		Skills:     skills,
	}, nil
}

func (c *HTTPClient) ResolveIDs(ctx context.Context, projectName, activityName, skillName string, options ResolveOptions) (ResolvedIDs, error) {
	snapshot, err := c.FetchLookupSnapshot(ctx)
	if err != nil {
		return ResolvedIDs{}, err
	}
	return ResolveIDsFromSnapshot(snapshot, projectName, activityName, skillName, options)
}

func ResolveIDsFromSnapshot(snapshot LookupSnapshot, projectName, activityName, skillName string, options ResolveOptions) (ResolvedIDs, error) {
	projectName = normalize(projectName)
	activityName = normalize(activityName)
	skillName = normalize(skillName)
	if projectName == "" || activityName == "" || skillName == "" {
		return ResolvedIDs{}, errors.New("project, activity and skill names are required")
	}

	projectCandidates := make([]Project, 0)
	archivedOnly := make([]Project, 0)
	for _, project := range snapshot.Projects {
		if !equalName(project.Name, projectName) {
			continue
		}
		if project.IsArchived() {
			archivedOnly = append(archivedOnly, project)
			if !options.IncludeArchivedProjects {
				continue
			}
		}
		projectCandidates = append(projectCandidates, project)
	}
	projectCandidates = uniqueProjects(projectCandidates)

	if len(projectCandidates) == 0 {
		if !options.IncludeArchivedProjects && len(archivedOnly) > 0 {
			return ResolvedIDs{}, fmt.Errorf(
				"project %q only matches archived projects (ids: %s); set IncludeArchivedProjects to true if this is intended",
				projectName,
				idsForProjects(archivedOnly),
			)
		}
		return ResolvedIDs{}, fmt.Errorf("project %q not found", projectName)
	}
	if len(projectCandidates) > 1 {
		return ResolvedIDs{}, fmt.Errorf("project %q is ambiguous (ids: %s)", projectName, idsForProjects(projectCandidates))
	}
	project := projectCandidates[0]

	activityCandidates := make([]Activity, 0)
	lockedOnly := make([]Activity, 0)
	for _, activity := range snapshot.Activities {
		if activity.ProjectNodeID != project.ID || !equalName(activity.Name, activityName) {
			continue
		}
		if activity.Locked {
			lockedOnly = append(lockedOnly, activity)
			if !options.IncludeLockedActivities {
				continue
			}
		}
		activityCandidates = append(activityCandidates, activity)
	}
	activityCandidates = uniqueActivities(activityCandidates)

	if len(activityCandidates) == 0 {
		if !options.IncludeLockedActivities && len(lockedOnly) > 0 {
			return ResolvedIDs{}, fmt.Errorf(
				"activity %q on project %q only matches locked activities (ids: %s); set IncludeLockedActivities to true if this is intended",
				activityName,
				project.Name,
				idsForActivities(lockedOnly),
			)
		}
		return ResolvedIDs{}, fmt.Errorf("activity %q not found on project %q", activityName, project.Name)
	}
	if len(activityCandidates) > 1 {
		return ResolvedIDs{}, fmt.Errorf(
			"activity %q on project %q is ambiguous (ids: %s)",
			activityName,
			project.Name,
			idsForActivities(activityCandidates),
		)
	}
	activity := activityCandidates[0]

	skillCandidates := make([]Skill, 0)
	for _, skill := range snapshot.Skills {
		if skill.ActivityID == activity.ID && equalName(skill.Name, skillName) {
			skillCandidates = append(skillCandidates, skill)
		}
	}
	skillCandidates = uniqueSkills(skillCandidates)

	if len(skillCandidates) == 0 {
		return ResolvedIDs{}, fmt.Errorf("skill %q not found for activity %q (id %d)", skillName, activity.Name, activity.ID)
	}
	if len(skillCandidates) > 1 {
		return ResolvedIDs{}, fmt.Errorf(
			"skill %q for activity %q (id %d) is ambiguous (skill ids: %s)",
			skillName,
			activity.Name,
			activity.ID,
			idsForSkills(skillCandidates),
		)
	}
	skill := skillCandidates[0]

	return ResolvedIDs{
		ProjectID:    project.ID,
		ActivityID:   activity.ID,
		SkillID:      skill.SkillID,
		ProjectName:  project.Name,
		ActivityName: activity.Name,
		SkillName:    skill.Name,
	}, nil
}

func FormatDay(day time.Time) string {
	return day.Format(dayLayout)
}

func ParseDay(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(dayLayout, strings.TrimSpace(value), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse day %q: %w", value, err)
	}
	return parsed, nil
}

func (c *HTTPClient) doJSON(ctx context.Context, method, endpointPath string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	url := c.baseURL + endpointPath
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request %s %s: %w", method, endpointPath, err)
	}

	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.refererURL != "" {
		req.Header.Set("Referer", c.refererURL)
	}
	if c.sessionCookies != "" {
		req.Header.Set("Cookie", c.sessionCookies)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s failed: %w", method, endpointPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf(
			"request %s %s failed with status %d: %s",
			method,
			endpointPath,
			resp.StatusCode,
			strings.TrimSpace(string(responseBody)),
		)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decode response %s %s: %w", method, endpointPath, err)
	}
	return nil
}

func equalName(a, b string) bool {
	return strings.EqualFold(normalize(a), normalize(b))
}

func normalize(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func uniqueProjects(values []Project) []Project {
	seen := make(map[int64]struct{}, len(values))
	result := make([]Project, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value.ID]; ok {
			continue
		}
		seen[value.ID] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueActivities(values []Activity) []Activity {
	seen := make(map[int64]struct{}, len(values))
	result := make([]Activity, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value.ID]; ok {
			continue
		}
		seen[value.ID] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueSkills(values []Skill) []Skill {
	seen := make(map[int64]struct{}, len(values))
	result := make([]Skill, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value.SkillID]; ok {
			continue
		}
		seen[value.SkillID] = struct{}{}
		result = append(result, value)
	}
	return result
}

func idsForProjects(values []Project) string {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return formatIDs(ids)
}

func idsForActivities(values []Activity) string {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return formatIDs(ids)
}

func idsForSkills(values []Skill) string {
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.SkillID)
	}
	return formatIDs(ids)
}

func formatIDs(values []int64) string {
	unique := make(map[int64]struct{}, len(values))
	for _, value := range values {
		unique[value] = struct{}{}
	}

	sorted := make([]int64, 0, len(unique))
	for value := range unique {
		sorted = append(sorted, value)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	items := make([]string, 0, len(sorted))
	for _, value := range sorted {
		items = append(items, strconv.FormatInt(value, 10))
	}
	return strings.Join(items, ", ")
}
