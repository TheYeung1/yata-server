package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/TheYeung1/yata-server/config"
	"github.com/TheYeung1/yata-server/database"
	"github.com/TheYeung1/yata-server/middleware/auth"
	"github.com/TheYeung1/yata-server/model"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	CognitoCfg config.AwsCognitoUserPoolConfig
	Ydb        database.YataDatabase
}

type InsertListItemInput struct {
	ItemID  string
	Content string
}

func writeInternalErrorResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("Sorry! Something went wrong"))
}

func (s *Server) GetList(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)

	userID, err := getUserIDFromContext(r)
	if err != nil {
		log.WithError(err).Error("failed to get user ID from request context")
		writeInternalErrorResponse(w)
		return
	}
	listID := model.ListID(v["listID"])

	yl, err := s.Ydb.GetList(userID, listID)
	if err != nil {
		if lnf, ok := err.(database.ListNotFoundError); ok {
			log.WithError(lnf).Info("list not found")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("List not found"))
			return
		}
		log.WithError(err).Error("failed to get list")
		writeInternalErrorResponse(w)
		return
	}

	res, err := json.Marshal(yl)
	if err != nil {
		log.WithError(err).Error("failed to marshal json")
		writeInternalErrorResponse(w)
	}
	_, err = w.Write(res)
	if err != nil {
		log.WithError(err).Error("failed to write response")
	}
}

func (s *Server) GetLists(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r)
	if err != nil {
		log.WithError(err).Error("failed to get user ID from request context")
		writeInternalErrorResponse(w)
		return
	}

	yl, err := s.Ydb.GetLists(userID)
	if err != nil {
		log.WithError(err).Error("failed to get lists")
		writeInternalErrorResponse(w)
	}

	res, err := json.Marshal(yl)
	if err != nil {
		log.WithError(err).Error("failed to marshal json")
		writeInternalErrorResponse(w)
	}

	_, err = w.Write(res)
	if err != nil {
		log.WithError(err).Error("failed to write response")
	}
}

func (s *Server) InsertListItem(w http.ResponseWriter, r *http.Request) {
	//TODO: add validation to inputs
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("failed to read request")
		writeInternalErrorResponse(w)
		return
	}

	var in InsertListItemInput
	err = json.Unmarshal(b, &in)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal request")
		writeInternalErrorResponse(w)
		return
	}

	uid, err := getUserIDFromContext(r)
	if err != nil {
		log.WithError(err).Error("failed to get user ID from request context")
		writeInternalErrorResponse(w)
		return
	}
	v := mux.Vars(r)

	// TODO: assert input lengths
	item := model.YataItem{
		UserID:  model.UserID(uid[0]),
		ListID:  model.ListID(v["listID"]),
		ItemID:  model.ItemID(in.ItemID),
		Content: in.Content,
	}

	// insert list to db here
	err = s.Ydb.InsertItem(item)
	if err != nil {
		log.WithError(err).Error("failed to insert item")
		writeInternalErrorResponse(w)
	}

	w.WriteHeader(http.StatusCreated)
	_, err = w.Write([]byte{})
	if err != nil {
		log.WithError(err).Error("failed to write response")
	}
}

func (s *Server) GetListItems(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)

	userID, err := getUserIDFromContext(r)
	if err != nil {
		log.WithError(err).Error("failed to get user ID from request context")
		writeInternalErrorResponse(w)
		return
	}
	listID := model.ListID(v["listID"])

	items, err := s.Ydb.GetListItems(userID, listID)
	if err != nil {
		log.WithError(err).Error("failed to get list items")
		writeInternalErrorResponse(w)
	}

	res, err := json.Marshal(items)
	if err != nil {
		log.WithError(err).Error("failed to marshal json")
		writeInternalErrorResponse(w)
	}

	_, err = w.Write(res)
	if err != nil {
		log.WithError(err).Error("failed to write response")
	}
}

func (s *Server) GetAllItems(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromContext(r)
	if err != nil {
		log.WithError(err).Error("failed to get user ID from request context")
		writeInternalErrorResponse(w)
		return
	}

	items, err := s.Ydb.GetAllItems(userID)
	if err != nil {
		log.WithError(err).Error("failed to get all items")
		writeInternalErrorResponse(w)
	}

	res, err := json.Marshal(items)
	if err != nil {
		log.WithError(err).Error("failed to marshal json")
		writeInternalErrorResponse(w)
	}

	_, err = w.Write(res)
	if err != nil {
		log.WithError(err).Error("failed to write response")
	}
}

func (s *Server) Start() {
	addr := ":8888"
	log.WithField("address", addr).Info("starting server")
	r := mux.NewRouter()
	r.Use(auth.CognitoJwtAuthMiddleware{Cfg: s.CognitoCfg}.Execute)
	r.HandleFunc("/items", s.GetAllItems).Methods(http.MethodGet)
	r.HandleFunc("/lists", s.GetLists).Methods(http.MethodGet)
	r.HandleFunc("/lists", s.InsertList).Methods(http.MethodPut)
	r.HandleFunc("/lists-alternate", NewInsertListHandler(s.Ydb.InsertList)).Methods(http.MethodPut)
	r.HandleFunc("/lists/{listID}/", s.GetList).Methods(http.MethodGet)
	r.HandleFunc("/lists/{listID}/items", s.GetListItems).Methods(http.MethodGet)
	r.HandleFunc("/lists/{listID}/items", s.InsertListItem).Methods(http.MethodPut)
	log.Fatal(http.ListenAndServe(addr, r))
}
