package api

import (
	"io"
	"net/http"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
)

// RelationshipHandler provides admin API endpoints for managing SpiceDB relationships.
type RelationshipHandler struct {
	client *authzed.Client
}

func NewRelationshipHandler(client *authzed.Client) *RelationshipHandler {
	return &RelationshipHandler{client: client}
}

func (h *RelationshipHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/admin/relationships", h.handleCreate)
	mux.HandleFunc("DELETE /api/admin/relationships", h.handleDelete)
	mux.HandleFunc("GET /api/admin/relationships/lookup", h.handleLookup)
	mux.HandleFunc("GET /api/admin/schema/definitions", h.handleDefinitions)
}

type relationshipRequest struct {
	ObjectType  string `json:"object_type"`
	ObjectID    string `json:"object_id"`
	Relation    string `json:"relation"`
	SubjectType string `json:"subject_type"`
	SubjectID   string `json:"subject_id"`
}

func (r *relationshipRequest) validate() string {
	if r.ObjectType == "" {
		return "object_type is required"
	}
	if r.ObjectID == "" {
		return "object_id is required"
	}
	if r.Relation == "" {
		return "relation is required"
	}
	if r.SubjectType == "" {
		return "subject_type is required"
	}
	if r.SubjectID == "" {
		return "subject_id is required"
	}
	return ""
}

func (r *relationshipRequest) toRelationship() *v1.Relationship {
	return &v1.Relationship{
		Resource: &v1.ObjectReference{ObjectType: r.ObjectType, ObjectId: r.ObjectID},
		Relation: r.Relation,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{ObjectType: r.SubjectType, ObjectId: r.SubjectID},
		},
	}
}

// POST /api/admin/relationships — create a relationship
func (h *RelationshipHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req relationshipRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	_, err := h.client.WriteRelationships(r.Context(), &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
				Relationship: req.toRelationship(),
			},
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write relationship: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// DELETE /api/admin/relationships — delete a relationship
func (h *RelationshipHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req relationshipRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	_, err := h.client.WriteRelationships(r.Context(), &v1.WriteRelationshipsRequest{
		Updates: []*v1.RelationshipUpdate{
			{
				Operation:    v1.RelationshipUpdate_OPERATION_DELETE,
				Relationship: req.toRelationship(),
			},
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete relationship: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type lookupResult struct {
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
}

// GET /api/admin/relationships/lookup?resource_type=study_group&permission=view_own_data&subject_type=user&subject_id=123
func (h *RelationshipHandler) handleLookup(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	permission := r.URL.Query().Get("permission")
	subjectType := r.URL.Query().Get("subject_type")
	subjectID := r.URL.Query().Get("subject_id")

	if resourceType == "" || permission == "" || subjectType == "" || subjectID == "" {
		writeError(w, http.StatusBadRequest, "resource_type, permission, subject_type, and subject_id are required")
		return
	}

	stream, err := h.client.LookupResources(r.Context(), &v1.LookupResourcesRequest{
		ResourceObjectType: resourceType,
		Permission:         permission,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{ObjectType: subjectType, ObjectId: subjectID},
		},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup failed: "+err.Error())
		return
	}

	var results []lookupResult
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		results = append(results, lookupResult{
			ObjectType: resourceType,
			ObjectID:   resp.ResourceObjectId,
		})
	}

	writeJSON(w, http.StatusOK, results)
}

type definitionInfo struct {
	Name      string   `json:"name"`
	Relations []string `json:"relations,omitempty"`
}

// GET /api/admin/schema/definitions — list all SpiceDB definitions from schema
func (h *RelationshipHandler) handleDefinitions(w http.ResponseWriter, r *http.Request) {
	resp, err := h.client.ReadSchema(r.Context(), &v1.ReadSchemaRequest{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read schema: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"schema": resp.SchemaText})
}
