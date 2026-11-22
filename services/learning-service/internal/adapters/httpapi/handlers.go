package httpapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/app"
	"github.com/JohnnyMatteus/people-platform/services/learning-service/internal/domain"
)

type itemDTO struct {
	ID         uuid.UUID `json:"id"`
	Position   int       `json:"position"`
	Title      string    `json:"title"`
	Kind       string    `json:"kind"`
	Minutes    int       `json:"minutes"`
	Competency string    `json:"competency,omitempty"`
}

type trackDTO struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Minutes     int       `json:"minutes"`
	Items       []itemDTO `json:"items"`
}

func toTrack(t domain.Track) trackDTO {
	items := make([]itemDTO, 0, len(t.Items))
	for _, i := range t.Items {
		items = append(items, itemDTO{ID: i.ID, Position: i.Position, Title: i.Title, Kind: string(i.Kind), Minutes: i.Minutes, Competency: i.Competency})
	}
	return trackDTO{ID: t.ID, Title: t.Title, Description: t.Description, Minutes: t.Minutes(), Items: items}
}

type enrollmentDTO struct {
	ID         uuid.UUID            `json:"id"`
	TrackID    uuid.UUID            `json:"track_id"`
	EmployeeID uuid.UUID            `json:"employee_id"`
	Source     string               `json:"source"`
	EnrolledAt time.Time            `json:"enrolled_at"`
	Track      *trackDTO            `json:"track,omitempty"`
	Done       *int                 `json:"done_items,omitempty"`
	Total      *int                 `json:"total_items,omitempty"`
	Next       *itemDTO             `json:"next_item,omitempty"`
	Completed  map[string]time.Time `json:"completed,omitempty"`
}

func toEnrollment(e domain.Enrollment) enrollmentDTO {
	return enrollmentDTO{ID: e.ID, TrackID: e.TrackID, EmployeeID: e.EmployeeID, Source: string(e.Source), EnrolledAt: e.EnrolledAt}
}

func toEnrollmentView(v app.EnrollmentView) enrollmentDTO {
	dto := toEnrollment(v.Enrollment)
	track := toTrack(v.Track)
	dto.Track, dto.Done, dto.Total = &track, &v.Done, &v.Total
	dto.Completed = make(map[string]time.Time, len(v.Enrollment.Completed))
	for id, at := range v.Enrollment.Completed {
		dto.Completed[id.String()] = at
	}
	if v.Next != nil {
		next := itemDTO{ID: v.Next.ID, Position: v.Next.Position, Title: v.Next.Title, Kind: string(v.Next.Kind), Minutes: v.Next.Minutes, Competency: v.Next.Competency}
		dto.Next = &next
	}
	return dto
}

type suggestionDTO struct {
	TrackID    uuid.UUID `json:"track_id"`
	TrackTitle string    `json:"track_title"`
	Item       itemDTO   `json:"item"`
	Reason     string    `json:"reason"`
}

func toSuggestions(list []domain.Suggestion) []suggestionDTO {
	out := make([]suggestionDTO, 0, len(list))
	for _, s := range list {
		out = append(out, suggestionDTO{
			TrackID: s.Track.ID, TrackTitle: s.Track.Title, Reason: s.Reason,
			Item: itemDTO{ID: s.Item.ID, Position: s.Item.Position, Title: s.Item.Title, Kind: string(s.Item.Kind), Minutes: s.Item.Minutes, Competency: s.Item.Competency},
		})
	}
	return out
}

type onboardingDTO struct {
	ApplicationID uuid.UUID `json:"application_id"`
	PositionTitle string    `json:"position_title"`
	Unit          string    `json:"unit"`
	StartDate     string    `json:"start_date"`
}

func (s *Server) listTracks(w http.ResponseWriter, r *http.Request, p app.Principal) {
	tracks, err := s.svc.ListTracks(r.Context(), p)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]trackDTO, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, toTrack(t))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createTrack(w http.ResponseWriter, r *http.Request, p app.Principal) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decode(r, &body); err != nil {
		s.writeError(w, r, err)
		return
	}
	track, err := s.svc.CreateTrack(r.Context(), p, body.Title, body.Description)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTrack(track))
}

func (s *Server) getTrack(w http.ResponseWriter, r *http.Request, p app.Principal) {
	id, err := pathUUID(r, "trackId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	track, err := s.svc.GetTrack(r.Context(), p, id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toTrack(track))
}

func (s *Server) addItem(w http.ResponseWriter, r *http.Request, p app.Principal) {
	id, err := pathUUID(r, "trackId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var body struct {
		Title      string `json:"title"`
		Kind       string `json:"kind"`
		Minutes    int    `json:"minutes"`
		Competency string `json:"competency"`
	}
	if err := decode(r, &body); err != nil {
		s.writeError(w, r, err)
		return
	}
	track, err := s.svc.AddItem(r.Context(), p, id, body.Title, domain.ItemKind(body.Kind), body.Minutes, body.Competency)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTrack(track))
}

func (s *Server) enroll(w http.ResponseWriter, r *http.Request, p app.Principal) {
	id, err := pathUUID(r, "trackId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var body struct {
		EmployeeID *uuid.UUID `json:"employee_id"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &body); err != nil {
			s.writeError(w, r, err)
			return
		}
	}
	e, err := s.svc.Enroll(r.Context(), p, id, body.EmployeeID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEnrollment(e))
}

func (s *Server) myEnrollments(w http.ResponseWriter, r *http.Request, p app.Principal) {
	views, err := s.svc.MyEnrollments(r.Context(), p)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]enrollmentDTO, 0, len(views))
	for _, v := range views {
		out = append(out, toEnrollmentView(v))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) complete(w http.ResponseWriter, r *http.Request, p app.Principal) {
	enrollmentID, err := pathUUID(r, "enrollmentId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	itemID, err := pathUUID(r, "itemId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	view, err := s.svc.Complete(r.Context(), p, enrollmentID, itemID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEnrollmentView(view))
}

func (s *Server) suggestions(w http.ResponseWriter, r *http.Request, p app.Principal) {
	next, byCompetency, err := s.svc.Suggestions(r.Context(), p, r.URL.Query().Get("competency"))
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"next": toSuggestions(next), "for_competency": toSuggestions(byCompetency)})
}

func (s *Server) getDefaultTrack(w http.ResponseWriter, r *http.Request, p app.Principal) {
	id, err := s.svc.DefaultOnboardingTrack(r.Context(), p)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]*uuid.UUID{"track_id": id})
}

func (s *Server) putDefaultTrack(w http.ResponseWriter, r *http.Request, p app.Principal) {
	var body struct {
		TrackID uuid.UUID `json:"track_id"`
	}
	if err := decode(r, &body); err != nil {
		s.writeError(w, r, err)
		return
	}
	if err := s.svc.SetDefaultOnboardingTrack(r.Context(), p, body.TrackID); err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]uuid.UUID{"track_id": body.TrackID})
}

func (s *Server) pendingOnboarding(w http.ResponseWriter, r *http.Request, p app.Principal) {
	list, err := s.svc.PendingOnboarding(r.Context(), p)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	out := make([]onboardingDTO, 0, len(list))
	for _, a := range list {
		out = append(out, onboardingDTO{ApplicationID: a.ApplicationID, PositionTitle: a.PositionTitle, Unit: a.Unit, StartDate: a.StartDate.Format("2006-01-02")})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) bindOnboarding(w http.ResponseWriter, r *http.Request, p app.Principal) {
	applicationID, err := pathUUID(r, "applicationId")
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	var body struct {
		EmployeeID uuid.UUID `json:"employee_id"`
	}
	if err := decode(r, &body); err != nil {
		s.writeError(w, r, err)
		return
	}
	e, err := s.svc.BindOnboarding(r.Context(), p, applicationID, body.EmployeeID)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEnrollment(e))
}
