package handler

import (
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"taxpod-crm/api/internal/model"
	"taxpod-crm/api/internal/repository"
	"taxpod-crm/api/internal/service"
)

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var moneyPattern = regexp.MustCompile(`^\d{1,12}(\.\d{1,2})?$`)

type CRMHandler struct{ crm *service.CRMService }

func NewCRMHandler(crm *service.CRMService) *CRMHandler { return &CRMHandler{crm: crm} }

func (h *CRMHandler) Register(r gin.IRoutes) {
	r.GET("/contacts", h.ListContacts)
	r.POST("/contacts", h.CreateContact)
	r.GET("/contacts/:id", h.Contact)
	r.PUT("/contacts/:id", h.UpdateContact)
	r.DELETE("/contacts/:id", h.DeleteContact)
	r.GET("/opportunities", h.ListOpportunities)
	r.POST("/opportunities", h.CreateOpportunity)
	r.PUT("/opportunities/:id", h.UpdateOpportunity)
	r.PATCH("/opportunities/:id/stage", h.MoveOpportunity)
	r.DELETE("/opportunities/:id", h.DeleteOpportunity)
	r.GET("/activities", h.ListActivities)
	r.POST("/activities", h.CreateActivity)
	r.GET("/dashboard", h.Dashboard)
}

func (h *CRMHandler) ListContacts(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	if status != "" && !oneOf(status, "lead", "prospect", "customer") {
		writeError(c, 400, "VALIDATION_ERROR", "status must be lead, prospect, or customer.")
		return
	}
	result, err := h.crm.ListContacts(c.Request.Context(), strings.TrimSpace(c.Query("q")), status, page, size)
	if err != nil {
		writeInternal(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *CRMHandler) CreateContact(c *gin.Context) {
	var input struct {
		Name    string  `json:"name"`
		Email   string  `json:"email"`
		Phone   *string `json:"phone"`
		Company *string `json:"company"`
		Status  string  `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	input.Name, input.Email, input.Status = strings.TrimSpace(input.Name), strings.ToLower(strings.TrimSpace(input.Email)), strings.ToLower(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "lead"
	}
	if err := validateContact(input.Name, input.Email, input.Phone, input.Company, input.Status); err != "" {
		writeError(c, 400, "VALIDATION_ERROR", err)
		return
	}
	item, err := h.crm.CreateContact(c.Request.Context(), model.Contact{Name: input.Name, Email: input.Email, Phone: input.Phone, Company: input.Company, Status: input.Status})
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CRMHandler) Contact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	item, err := h.crm.Contact(c.Request.Context(), id)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CRMHandler) UpdateContact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input struct {
		Name    *string `json:"name"`
		Email   *string `json:"email"`
		Phone   *string `json:"phone"`
		Company *string `json:"company"`
		Status  *string `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	if input.Name == nil && input.Email == nil && input.Phone == nil && input.Company == nil && input.Status == nil {
		writeError(c, 400, "VALIDATION_ERROR", "Provide at least one field to update.")
		return
	}
	if input.Name != nil {
		*input.Name = strings.TrimSpace(*input.Name)
	}
	if input.Email != nil {
		*input.Email = strings.ToLower(strings.TrimSpace(*input.Email))
	}
	if input.Status != nil {
		*input.Status = strings.ToLower(strings.TrimSpace(*input.Status))
	}
	if input.Name != nil && (*input.Name == "" || len([]rune(*input.Name)) > 200) {
		writeError(c, 400, "VALIDATION_ERROR", "name must contain 1 to 200 characters.")
		return
	}
	if input.Email != nil && !validEmail(*input.Email) {
		writeError(c, 400, "VALIDATION_ERROR", "email must be a valid address of at most 320 characters.")
		return
	}
	if input.Phone != nil && len([]rune(*input.Phone)) > 50 {
		writeError(c, 400, "VALIDATION_ERROR", "phone must be at most 50 characters.")
		return
	}
	if input.Company != nil && len([]rune(*input.Company)) > 200 {
		writeError(c, 400, "VALIDATION_ERROR", "company must be at most 200 characters.")
		return
	}
	if input.Status != nil && !oneOf(*input.Status, "lead", "prospect", "customer") {
		writeError(c, 400, "VALIDATION_ERROR", "status must be lead, prospect, or customer.")
		return
	}
	item, err := h.crm.UpdateContact(c.Request.Context(), id, input.Name, input.Email, input.Phone, input.Company, input.Status)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CRMHandler) DeleteContact(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if _, _, err := h.crm.DeleteContact(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CRMHandler) ListOpportunities(c *gin.Context) {
	stage := strings.ToLower(strings.TrimSpace(c.Query("stage")))
	contactID := strings.TrimSpace(c.Query("contact_id"))
	if stage != "" && !validStage(stage) {
		writeError(c, 400, "VALIDATION_ERROR", "stage is not a supported opportunity stage.")
		return
	}
	if contactID != "" && !uuidPattern.MatchString(contactID) {
		writeError(c, 400, "VALIDATION_ERROR", "contact_id must be a UUID.")
		return
	}
	items, err := h.crm.ListOpportunities(c.Request.Context(), stage, contactID)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	if items == nil {
		items = []model.Opportunity{}
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *CRMHandler) CreateOpportunity(c *gin.Context) {
	var input struct {
		ContactID string `json:"contact_id"`
		Title     string `json:"title"`
		ValueRM   string `json:"value_rm"`
		Stage     string `json:"stage"`
		Position  int    `json:"position"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	input.ContactID, input.Title, input.Stage = strings.TrimSpace(input.ContactID), strings.TrimSpace(input.Title), strings.ToLower(strings.TrimSpace(input.Stage))
	if input.Stage == "" {
		input.Stage = "new"
	}
	if input.ValueRM == "" {
		input.ValueRM = "0"
	}
	if !uuidPattern.MatchString(input.ContactID) || input.Title == "" || len([]rune(input.Title)) > 200 || !validMoney(input.ValueRM) || !validStage(input.Stage) || input.Position < 0 {
		writeError(c, 400, "VALIDATION_ERROR", "contact_id, title, value_rm, stage, or position is invalid.")
		return
	}
	item, err := h.crm.CreateOpportunity(c.Request.Context(), model.Opportunity{ContactID: input.ContactID, Title: input.Title, ValueRM: input.ValueRM, Stage: input.Stage, Position: input.Position})
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CRMHandler) UpdateOpportunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input struct {
		Title    *string `json:"title"`
		ValueRM  *string `json:"value_rm"`
		Stage    *string `json:"stage"`
		Position *int    `json:"position"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	if input.Title == nil && input.ValueRM == nil && input.Stage == nil && input.Position == nil {
		writeError(c, 400, "VALIDATION_ERROR", "Provide at least one field to update.")
		return
	}
	if input.Title != nil {
		*input.Title = strings.TrimSpace(*input.Title)
		if *input.Title == "" || len([]rune(*input.Title)) > 200 {
			writeError(c, 400, "VALIDATION_ERROR", "title must contain 1 to 200 characters.")
			return
		}
	}
	if input.ValueRM != nil && !validMoney(*input.ValueRM) {
		writeError(c, 400, "VALIDATION_ERROR", "value_rm must be a non-negative amount with at most two decimal places.")
		return
	}
	if input.Stage != nil {
		*input.Stage = strings.ToLower(strings.TrimSpace(*input.Stage))
		if !validStage(*input.Stage) {
			writeError(c, 400, "VALIDATION_ERROR", "stage is not a supported opportunity stage.")
			return
		}
	}
	if input.Position != nil && *input.Position < 0 {
		writeError(c, 400, "VALIDATION_ERROR", "position cannot be negative.")
		return
	}
	item, err := h.crm.UpdateOpportunity(c.Request.Context(), id, input.Title, input.ValueRM, input.Stage, input.Position)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CRMHandler) MoveOpportunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var input struct {
		Stage    string `json:"stage"`
		Position *int   `json:"position"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	input.Stage = strings.ToLower(strings.TrimSpace(input.Stage))
	if !validStage(input.Stage) || (input.Position != nil && *input.Position < 0) {
		writeError(c, 400, "VALIDATION_ERROR", "stage or position is invalid.")
		return
	}
	item, err := h.crm.MoveOpportunity(c.Request.Context(), id, input.Stage, input.Position)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *CRMHandler) DeleteOpportunity(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.crm.DeleteOpportunity(c.Request.Context(), id); err != nil {
		writeDatabaseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *CRMHandler) ListActivities(c *gin.Context) {
	contactID, opportunityID := strings.TrimSpace(c.Query("contact_id")), strings.TrimSpace(c.Query("opportunity_id"))
	if (contactID != "" && !uuidPattern.MatchString(contactID)) || (opportunityID != "" && !uuidPattern.MatchString(opportunityID)) {
		writeError(c, 400, "VALIDATION_ERROR", "contact_id and opportunity_id must be UUIDs.")
		return
	}
	items, err := h.crm.ListActivities(c.Request.Context(), contactID, opportunityID)
	if err != nil {
		writeDatabaseError(c, err)
		return
	}
	if items == nil {
		items = []model.Activity{}
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *CRMHandler) CreateActivity(c *gin.Context) {
	var input struct {
		Type          string  `json:"type"`
		ContactID     *string `json:"contact_id"`
		OpportunityID *string `json:"opportunity_id"`
		OccurredAt    string  `json:"occurred_at"`
		Description   string  `json:"description"`
	}
	if c.ShouldBindJSON(&input) != nil {
		writeError(c, 400, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	input.Type, input.Description = strings.ToLower(strings.TrimSpace(input.Type)), strings.TrimSpace(input.Description)
	if !oneOf(input.Type, "call", "email", "meeting", "note") || (input.ContactID == nil && input.OpportunityID == nil) || input.Description == "" || len([]rune(input.Description)) > 5000 {
		writeError(c, 400, "VALIDATION_ERROR", "type, an owner, and a description of at most 5000 characters are required.")
		return
	}
	if input.ContactID != nil && !uuidPattern.MatchString(*input.ContactID) || input.OpportunityID != nil && !uuidPattern.MatchString(*input.OpportunityID) {
		writeError(c, 400, "VALIDATION_ERROR", "contact_id and opportunity_id must be UUIDs.")
		return
	}
	occurredAt := time.Now().UTC()
	if input.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, input.OccurredAt)
		if err != nil {
			writeError(c, 400, "VALIDATION_ERROR", "occurred_at must be RFC 3339.")
			return
		}
		occurredAt = parsed
	}
	item, err := h.crm.CreateActivity(c.Request.Context(), model.Activity{Type: input.Type, ContactID: input.ContactID, OpportunityID: input.OpportunityID, OccurredAt: occurredAt, Description: input.Description})
	if err != nil {
		if repository.IsNotFound(err) {
			writeError(c, 400, "VALIDATION_ERROR", "The activity owner does not exist or does not match.")
		} else {
			writeDatabaseError(c, err)
		}
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *CRMHandler) Dashboard(c *gin.Context) {
	result, err := h.crm.Dashboard(c.Request.Context())
	if err != nil {
		writeInternal(c)
		return
	}
	c.JSON(http.StatusOK, result)
}

func pagination(c *gin.Context) (int, int, bool) {
	page, size := 1, 20
	var err error
	if value := c.Query("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			writeError(c, 400, "VALIDATION_ERROR", "page must be a positive integer.")
			return 0, 0, false
		}
	}
	if value := c.Query("page_size"); value != "" {
		size, err = strconv.Atoi(value)
		if err != nil || size < 1 || size > 100 {
			writeError(c, 400, "VALIDATION_ERROR", "page_size must be between 1 and 100.")
			return 0, 0, false
		}
	}
	return page, size, true
}

func pathID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if !uuidPattern.MatchString(id) {
		writeError(c, 400, "VALIDATION_ERROR", "id must be a UUID.")
		return "", false
	}
	return id, true
}

func validateContact(name, email string, phone, company *string, status string) string {
	if name == "" || len([]rune(name)) > 200 {
		return "name must contain 1 to 200 characters."
	}
	if !validEmail(email) {
		return "email must be a valid address of at most 320 characters."
	}
	if phone != nil && len([]rune(*phone)) > 50 {
		return "phone must be at most 50 characters."
	}
	if company != nil && len([]rune(*company)) > 200 {
		return "company must be at most 200 characters."
	}
	if !oneOf(status, "lead", "prospect", "customer") {
		return "status must be lead, prospect, or customer."
	}
	return ""
}

func validEmail(value string) bool {
	if len(value) > 320 {
		return false
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}
func validMoney(value string) bool { return moneyPattern.MatchString(value) }
func validStage(value string) bool {
	return oneOf(value, "new", "contacted", "qualified", "proposal", "won", "lost")
}
func oneOf(value string, allowed ...string) bool {
	for _, option := range allowed {
		if value == option {
			return true
		}
	}
	return false
}

func writeInternal(c *gin.Context) { writeError(c, 500, "INTERNAL", "An unexpected error occurred.") }
func writeDatabaseError(c *gin.Context, err error) {
	if repository.IsNotFound(err) {
		writeError(c, 404, "NOT_FOUND", "The requested record was not found.")
		return
	}
	if repository.IsDuplicate(err) {
		writeError(c, 409, "CONFLICT", "A record with this email already exists.")
		return
	}
	if repository.IsInvalidDatabaseValue(err) {
		writeError(c, 400, "VALIDATION_ERROR", "One or more values are invalid.")
		return
	}
	writeInternal(c)
}
