package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/service/people"
)

// People is what the API needs from staff and roles (phase 10, Task 4a).
// *people.Service is the implementation. actorID is the staff member
// signed in: what they may hand out is what they hold (ADR-115).
type People interface {
	People(ctx context.Context, venueID int64) ([]people.Person, error)
	Roles(ctx context.Context, venueID int64) ([]people.Role, error)
	AddStaff(ctx context.Context, venueID, actorID int64, in people.StaffInput) (string, error)
	UpdateStaff(ctx context.Context, venueID, actorID int64, staffID string, in people.StaffInput) error
	SetStaffActive(ctx context.Context, venueID, actorID int64, staffID string, active bool) error
	ResetPIN(ctx context.Context, venueID, actorID int64, staffID, pin string) error
	ClearLock(ctx context.Context, venueID, actorID int64, staffID string) error
	AddRole(ctx context.Context, venueID, actorID int64, in people.RoleInput) (int64, error)
	UpdateRole(ctx context.Context, venueID, actorID, roleID int64, in people.RoleInput) error
	DeleteRole(ctx context.Context, venueID, actorID, roleID int64) error
}

// WithPeople mounts staff and roles. They need WithDevices and WithStaff,
// and staff.manage.
func WithPeople(p People) Option {
	return func(s *Server) { s.people = p }
}

// The people error codes.
const (
	CodeStaffNotFound ErrorCode = "staff_not_found"
	CodeRoleNotFound  ErrorCode = "role_not_found"
	CodeRoleInUse     ErrorCode = "role_in_use"
	CodeRoleNameTaken ErrorCode = "role_name_taken"
	CodeBeyondYou     ErrorCode = "beyond_your_permissions"
	CodeLastManager   ErrorCode = "last_staff_manager"
	CodeInvalidPerson ErrorCode = "invalid_person"
)

type (
	staffRequest = people.StaffInput
	roleRequest  = people.RoleInput
)

// pinRequest is a new PIN. It is never echoed or logged.
type pinRequest struct {
	PIN string `json:"pin"`
}

// peopleResponse is everyone and every role, in one read for the screen.
type peopleResponse struct {
	Staff []people.Person `json:"staff"`
	Roles []people.Role   `json:"roles"`
}

type staffAddedResponse struct {
	ID string `json:"id"`
}

type roleEditResponse struct {
	ID int64 `json:"id"`
}

func (s *Server) mountPeople(mux *http.ServeMux) {
	if s.devices == nil || s.staff == nil || s.people == nil {
		return
	}
	handle := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, s.requireDevice(s.requireStaff(s.requirePermission(auth.PermStaffManage, h))))
	}
	handle("GET /api/v1/people", s.listPeople)
	handle("POST /api/v1/people/staff", s.addStaff)
	handle("PUT /api/v1/people/staff/{id}", s.updateStaff)
	handle("POST /api/v1/people/staff/{id}/deactivate", s.activateStaff(false))
	handle("POST /api/v1/people/staff/{id}/reactivate", s.activateStaff(true))
	handle("PUT /api/v1/people/staff/{id}/pin", s.resetPIN)
	handle("POST /api/v1/people/staff/{id}/unlock", s.unlockStaff)
	handle("POST /api/v1/people/roles", s.addRole)
	handle("PUT /api/v1/people/roles/{id}", s.updateRole)
	handle("DELETE /api/v1/people/roles/{id}", s.deleteRole)
}

// actor is the staff member signed in.
func actor(r *http.Request) int64 {
	session, _ := SessionFrom(r.Context())
	return session.Staff.ID
}

func (s *Server) listPeople(w http.ResponseWriter, r *http.Request) {
	venueID := Meta(r).VenueID
	staff, err := s.people.People(r.Context(), venueID)
	if err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	roles, err := s.people.Roles(r.Context(), venueID)
	if err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, peopleResponse{Staff: staff, Roles: roles})
}

func (s *Server) addStaff(w http.ResponseWriter, r *http.Request) {
	var req staffRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.people.AddStaff(r.Context(), Meta(r).VenueID, actor(r), req)
	if err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, staffAddedResponse{ID: id})
}

func (s *Server) updateStaff(w http.ResponseWriter, r *http.Request) {
	var req staffRequest
	if !s.decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if err := s.people.UpdateStaff(r.Context(), Meta(r).VenueID, actor(r), id, req); err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, staffAddedResponse{ID: id})
}

func (s *Server) activateStaff(active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := s.people.SetStaffActive(r.Context(), Meta(r).VenueID, actor(r), id, active); err != nil {
			s.writePeopleError(w, r, err)
			return
		}
		writeJSON(w, s.log, http.StatusOK, staffAddedResponse{ID: id})
	}
}

func (s *Server) resetPIN(w http.ResponseWriter, r *http.Request) {
	var req pinRequest
	if !s.decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if err := s.people.ResetPIN(r.Context(), Meta(r).VenueID, actor(r), id, req.PIN); err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, staffAddedResponse{ID: id})
}

// unlockStaff clears a hard lock (ADR-260). A person not locked is no
// error: the request is safe to retry.
func (s *Server) unlockStaff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.people.ClearLock(r.Context(), Meta(r).VenueID, actor(r), id); err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, staffAddedResponse{ID: id})
}

func (s *Server) addRole(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.people.AddRole(r.Context(), Meta(r).VenueID, actor(r), req)
	if err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusCreated, roleEditResponse{ID: id})
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	if err := s.people.UpdateRole(r.Context(), Meta(r).VenueID, actor(r), id, req); err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, roleEditResponse{ID: id})
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	if err := s.people.DeleteRole(r.Context(), Meta(r).VenueID, actor(r), id); err != nil {
		s.writePeopleError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, roleEditResponse{ID: id})
}

func (s *Server) writePeopleError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *people.Invalid
	switch {
	case errors.Is(err, people.ErrRoleNameTaken) && errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusConflict, CodeRoleNameTaken, invalid.Message, invalid.Field)
	case errors.As(err, &invalid):
		writeFieldError(w, s.log, http.StatusUnprocessableEntity, CodeInvalidPerson, invalid.Message, invalid.Field)
	case errors.Is(err, people.ErrStaffNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeStaffNotFound, "No such person.")
	case errors.Is(err, people.ErrRoleNotFound):
		writeError(w, s.log, http.StatusNotFound, CodeRoleNotFound, "No such role.")
	case errors.Is(err, people.ErrRoleInUse):
		writeError(w, s.log, http.StatusConflict, CodeRoleInUse, "People hold that role. Move them to another first.")
	case errors.Is(err, people.ErrBeyondYou):
		writeError(w, s.log, http.StatusForbidden, CodeBeyondYou,
			"That reaches permissions you don't have. Only someone who holds them can give them or change those who do.")
	case errors.Is(err, people.ErrPeerPIN):
		writeError(w, s.log, http.StatusForbidden, CodeBeyondYou,
			"Only someone with more permissions than they have can reset their PIN. They can change their own.")
	case errors.Is(err, people.ErrLastManager):
		writeError(w, s.log, http.StatusConflict, CodeLastManager,
			"Nobody would be left who can manage staff. Give someone else that first.")
	default:
		s.internalError(w, r, "a people request failed", err)
	}
}
