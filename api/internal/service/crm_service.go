package service

import (
	"context"

	"taxpod-crm/api/internal/model"
	"taxpod-crm/api/internal/repository"
)

type CRMService struct{ repo *repository.CRMRepository }

func NewCRMService(repo *repository.CRMRepository) *CRMService { return &CRMService{repo: repo} }

func (s *CRMService) ListContacts(ctx context.Context, q, status string, page, size int) (model.Page[model.Contact], error) {
	return s.repo.ListContacts(ctx, q, status, page, size)
}
func (s *CRMService) CreateContact(ctx context.Context, item model.Contact) (model.Contact, error) {
	return s.repo.CreateContact(ctx, item)
}
func (s *CRMService) Contact(ctx context.Context, id string) (model.ContactDetail, error) {
	return s.repo.Contact(ctx, id)
}
func (s *CRMService) UpdateContact(ctx context.Context, id string, name, email, phone, company, status *string) (model.Contact, error) {
	return s.repo.UpdateContact(ctx, id, name, email, phone, company, status)
}
func (s *CRMService) DeleteContact(ctx context.Context, id string) (int64, int64, error) {
	return s.repo.DeleteContact(ctx, id)
}
func (s *CRMService) ListOpportunities(ctx context.Context, stage, contactID string) ([]model.Opportunity, error) {
	return s.repo.ListOpportunities(ctx, stage, contactID)
}
func (s *CRMService) CreateOpportunity(ctx context.Context, item model.Opportunity) (model.Opportunity, error) {
	return s.repo.CreateOpportunity(ctx, item)
}
func (s *CRMService) UpdateOpportunity(ctx context.Context, id string, title, valueRM, stage *string, position *int) (model.Opportunity, error) {
	return s.repo.UpdateOpportunity(ctx, id, title, valueRM, stage, position)
}
func (s *CRMService) MoveOpportunity(ctx context.Context, id, stage string, position *int) (model.Opportunity, error) {
	return s.repo.MoveOpportunity(ctx, id, stage, position)
}
func (s *CRMService) DeleteOpportunity(ctx context.Context, id string) error {
	return s.repo.DeleteOpportunity(ctx, id)
}
func (s *CRMService) ListActivities(ctx context.Context, contactID, opportunityID string) ([]model.Activity, error) {
	return s.repo.ListActivities(ctx, contactID, opportunityID)
}
func (s *CRMService) CreateActivity(ctx context.Context, item model.Activity) (model.Activity, error) {
	return s.repo.CreateActivity(ctx, item)
}
func (s *CRMService) Dashboard(ctx context.Context) (model.Dashboard, error) {
	return s.repo.Dashboard(ctx)
}
