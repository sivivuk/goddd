package booking

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	kitlog "github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"
	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"github.com/marcusolsson/goddd/cargo"
	"github.com/marcusolsson/goddd/location"
)

// MakeHandler returns a handler for the booking service.
func MakeHandler(ctx context.Context, bs Service, logger kitlog.Logger) http.Handler {
	opts := []kithttp.ServerOption{
		kithttp.ServerErrorLogger(logger),
		kithttp.ServerErrorEncoder(encodeError),
	}

	bookCargoHandler := kithttp.NewServer(
		ctx,
		makeBookCargoEndpoint(bs),
		decodeBookCargoRequest,
		encodeResponse,
		opts...,
	)
	loadCargoHandler := kithttp.NewServer(
		ctx,
		makeLoadCargoEndpoint(bs),
		decodeLoadCargoRequest,
		encodeResponse,
		opts...,
	)
	requestRoutesHandler := kithttp.NewServer(
		ctx,
		makeRequestRoutesEndpoint(bs),
		decodeRequestRoutesRequest,
		encodeResponse,
		opts...,
	)
	assignToRouteHandler := kithttp.NewServer(
		ctx,
		makeAssignToRouteEndpoint(bs),
		decodeAssignToRouteRequest,
		encodeResponse,
		opts...,
	)
	changeDestinationHandler := kithttp.NewServer(
		ctx,
		makeChangeDestinationEndpoint(bs),
		decodeChangeDestinationRequest,
		encodeResponse,
		opts...,
	)
	listCargosHandler := kithttp.NewServer(
		ctx,
		makeListCargosEndpoint(bs),
		decodeListCargosRequest,
		encodeResponse,
		opts...,
	)
	listLocationsHandler := kithttp.NewServer(
		ctx,
		makeListLocationsEndpoint(bs),
		decodeListLocationsRequest,
		encodeResponse,
		opts...,
	)

	r := mux.NewRouter()

	r.Handle("/booking/v1/cargos", bookCargoHandler).Methods("POST")
	r.Handle("/booking/v1/cargos", listCargosHandler).Methods("GET")
	r.Handle("/booking/v1/cargos/{id}", loadCargoHandler).Methods("GET")
	r.Handle("/booking/v1/cargos/{id}/request_routes", requestRoutesHandler).Methods("GET")
	r.Handle("/booking/v1/cargos/{id}/assign_to_route", assignToRouteHandler).Methods("POST")
	r.Handle("/booking/v1/cargos/{id}/change_destination", changeDestinationHandler).Methods("POST")
	r.Handle("/booking/v1/locations", listLocationsHandler).Methods("GET")
	r.Handle("/booking/v1/docs", http.StripPrefix("/booking/v1/docs", http.FileServer(http.Dir("booking/docs"))))

	return r
}

var errBadRoute = errors.New("bad route")

func decodeBookCargoRequest(r *http.Request) (interface{}, error) {
	var body struct {
		Origin          string    `json:"origin"`
		Destination     string    `json:"destination"`
		ArrivalDeadline time.Time `json:"arrival_deadline"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}

	return bookCargoRequest{
		Origin:          location.UNLocode(body.Origin),
		Destination:     location.UNLocode(body.Destination),
		ArrivalDeadline: body.ArrivalDeadline,
	}, nil
}

func decodeLoadCargoRequest(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		return nil, errBadRoute
	}
	return loadCargoRequest{ID: cargo.TrackingID(id)}, nil
}

func decodeRequestRoutesRequest(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		return nil, errBadRoute
	}
	return requestRoutesRequest{ID: cargo.TrackingID(id)}, nil
}

func decodeAssignToRouteRequest(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		return nil, errBadRoute
	}

	var itinerary cargo.Itinerary
	if err := json.NewDecoder(r.Body).Decode(&itinerary); err != nil {
		return nil, err
	}

	return assignToRouteRequest{
		ID:        cargo.TrackingID(id),
		Itinerary: itinerary,
	}, nil
}

func decodeChangeDestinationRequest(r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	id, ok := vars["id"]
	if !ok {
		return nil, errBadRoute
	}

	var body struct {
		Destination string `json:"destination"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}

	return changeDestinationRequest{
		ID:          cargo.TrackingID(id),
		Destination: location.UNLocode(body.Destination),
	}, nil
}

func decodeListCargosRequest(r *http.Request) (interface{}, error) {
	return listCargosRequest{}, nil
}

func decodeListLocationsRequest(r *http.Request) (interface{}, error) {
	return listLocationsRequest{}, nil
}

func encodeResponse(w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		encodeError(w, e.error())
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

type errorer interface {
	error() error
}

// encode errors from business-logic
func encodeError(w http.ResponseWriter, err error) {
	switch err {
	case nil:
		w.WriteHeader(http.StatusOK)
	case cargo.ErrUnknown:
		w.WriteHeader(http.StatusNotFound)
	case ErrInvalidArgument:
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}
