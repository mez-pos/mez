package api

import (
	"net/http"

	"github.com/byjoellobo/mez/internal/service/menus"
)

type (
	stationRequest = menus.StationInput
	courseRequest  = menus.CourseInput
	retireRequest  = menus.RetireInput
)

// stationRetiredResponse answers retiring a station with what it moved: how
// many items went to the station named, and whether its printer did.
type stationRetiredResponse struct {
	ID           int64 `json:"id"`
	ItemsMoved   int64 `json:"items_moved"`
	PrinterMoved bool  `json:"printer_moved"`
}

// courseRetiredResponse answers retiring a course with how many items went to
// the course named.
type courseRetiredResponse struct {
	ID         int64 `json:"id"`
	ItemsMoved int64 `json:"items_moved"`
}

// mountStationsAndCourses routes the stations and the courses under
// /api/v1/menu/, as the categories are (phase 10, Task 6e).
func (s *Server) mountStationsAndCourses(handle func(string, http.HandlerFunc)) {
	handle("POST /api/v1/menu/stations", s.createStation)
	// The literal path wins over {id}: "order" is never a station's id.
	handle("PUT /api/v1/menu/stations/order", s.orderStations)
	handle("PUT /api/v1/menu/stations/{id}", s.renameStation)
	handle("POST /api/v1/menu/stations/{id}/retire", s.retireStation)
	handle("POST /api/v1/menu/stations/{id}/restore", s.restoreStation)

	handle("POST /api/v1/menu/courses", s.createCourse)
	handle("PUT /api/v1/menu/courses/order", s.orderCourses)
	handle("PUT /api/v1/menu/courses/{id}", s.renameCourse)
	handle("POST /api/v1/menu/courses/{id}/retire", s.retireCourse)
	handle("POST /api/v1/menu/courses/{id}/restore", s.restoreCourse)
}

func (s *Server) createStation(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.menuEditor.CreateStation(r.Context(), Meta(r).VenueID, req)
	s.menuEdited(w, r, http.StatusCreated, id, err)
}

func (s *Server) renameStation(w http.ResponseWriter, r *http.Request) {
	var req stationRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.RenameStation(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) orderStations(w http.ResponseWriter, r *http.Request) {
	var req menuOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, 0, s.menuEditor.OrderStations(r.Context(), Meta(r).VenueID, req.IDs))
}

func (s *Server) retireStation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	req, ok := s.retireBody(w, r)
	if !ok {
		return
	}
	moved, err := s.menuEditor.RetireStation(r.Context(), Meta(r).VenueID, id, req)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, stationRetiredResponse{ID: id, ItemsMoved: moved.ItemsMoved, PrinterMoved: moved.PrinterMoved})
}

func (s *Server) restoreStation(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.RestoreStation(r.Context(), Meta(r).VenueID, id))
}

func (s *Server) createCourse(w http.ResponseWriter, r *http.Request) {
	var req courseRequest
	if !s.decode(w, r, &req) {
		return
	}
	id, err := s.menuEditor.CreateCourse(r.Context(), Meta(r).VenueID, req)
	s.menuEdited(w, r, http.StatusCreated, id, err)
}

func (s *Server) renameCourse(w http.ResponseWriter, r *http.Request) {
	var req courseRequest
	id, ok := s.pathID(w, r, "id")
	if !ok || !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.RenameCourse(r.Context(), Meta(r).VenueID, id, req))
}

func (s *Server) orderCourses(w http.ResponseWriter, r *http.Request) {
	var req menuOrderRequest
	if !s.decode(w, r, &req) {
		return
	}
	s.menuEdited(w, r, http.StatusOK, 0, s.menuEditor.OrderCourses(r.Context(), Meta(r).VenueID, req.IDs))
}

func (s *Server) retireCourse(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	req, ok := s.retireBody(w, r)
	if !ok {
		return
	}
	moved, err := s.menuEditor.RetireCourse(r.Context(), Meta(r).VenueID, id, req)
	if err != nil {
		s.writeMenuEditError(w, r, err)
		return
	}
	writeJSON(w, s.log, http.StatusOK, courseRetiredResponse{ID: id, ItemsMoved: moved.ItemsMoved})
}

func (s *Server) restoreCourse(w http.ResponseWriter, r *http.Request) {
	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}
	s.menuEdited(w, r, http.StatusOK, id, s.menuEditor.RestoreCourse(r.Context(), Meta(r).VenueID, id))
}

// retireBody reads where things go. The body may be left out, for a station
// or course that carries nothing: that is the same as naming none.
func (s *Server) retireBody(w http.ResponseWriter, r *http.Request) (retireRequest, bool) {
	var req retireRequest
	if r.ContentLength == 0 {
		return req, true
	}
	return req, s.decode(w, r, &req)
}
