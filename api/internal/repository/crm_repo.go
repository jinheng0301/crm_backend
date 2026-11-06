package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"taxpod-crm/api/internal/model"
)

type CRMRepository struct{ db *pgxpool.Pool }

func NewCRMRepository(db *pgxpool.Pool) *CRMRepository { return &CRMRepository{db: db} }

func (r *CRMRepository) ListContacts(ctx context.Context, q, status string, page, pageSize int) (model.Page[model.Contact], error) {
	result := model.Page[model.Contact]{Data: []model.Contact{}, Page: page, PageSize: pageSize}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM contacts
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%' OR coalesce(company, '') ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status = $2)`, q, status).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := r.db.Query(ctx, `SELECT id::text, name, email, phone, company, status, created_at, updated_at
		FROM contacts
		WHERE ($1 = '' OR name ILIKE '%' || $1 || '%' OR email ILIKE '%' || $1 || '%' OR coalesce(company, '') ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC LIMIT $3 OFFSET $4`, q, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var contact model.Contact
		if err := rows.Scan(&contact.ID, &contact.Name, &contact.Email, &contact.Phone, &contact.Company, &contact.Status, &contact.CreatedAt, &contact.UpdatedAt); err != nil {
			return result, err
		}
		result.Data = append(result.Data, contact)
	}
	return result, rows.Err()
}

func (r *CRMRepository) CreateContact(ctx context.Context, contact model.Contact) (model.Contact, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return contact, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO contacts (name, email, phone, company, status)
		VALUES ($1, lower($2), $3, $4, $5)
		RETURNING id::text, name, email, phone, company, status, created_at, updated_at`,
		contact.Name, contact.Email, contact.Phone, contact.Company, contact.Status).
		Scan(&contact.ID, &contact.Name, &contact.Email, &contact.Phone, &contact.Company, &contact.Status, &contact.CreatedAt, &contact.UpdatedAt)
	if err != nil {
		return contact, err
	}
	if err := addSystemActivity(ctx, tx, contact.ID, nil, "Contact created: "+contact.Name); err != nil {
		return contact, err
	}
	return contact, tx.Commit(ctx)
}

func (r *CRMRepository) Contact(ctx context.Context, id string) (model.ContactDetail, error) {
	var result model.ContactDetail
	err := r.db.QueryRow(ctx, `SELECT id::text, name, email, phone, company, status, created_at, updated_at,
		(SELECT count(*)::int FROM opportunities WHERE contact_id = contacts.id),
		(SELECT count(*)::int FROM activities WHERE contact_id = contacts.id)
		FROM contacts WHERE id = $1`, id).Scan(&result.ID, &result.Name, &result.Email, &result.Phone, &result.Company,
		&result.Status, &result.CreatedAt, &result.UpdatedAt, &result.OpportunityCount, &result.ActivityCount)
	if err != nil {
		return result, err
	}
	result.Opportunities, err = r.opportunities(ctx, `WHERE o.contact_id = $1`, id)
	if err != nil {
		return result, err
	}
	result.Activities, err = r.activities(ctx, `WHERE a.contact_id = $1`, id)
	return result, err
}

func (r *CRMRepository) UpdateContact(ctx context.Context, id string, name, email, phone, company, status *string) (model.Contact, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Contact{}, err
	}
	defer tx.Rollback(ctx)
	var contact model.Contact
	err = tx.QueryRow(ctx, `UPDATE contacts SET
		name = coalesce($2, name), email = lower(coalesce($3, email)), phone = coalesce($4, phone),
		company = coalesce($5, company), status = coalesce($6, status), updated_at = now()
		WHERE id = $1
		RETURNING id::text, name, email, phone, company, status, created_at, updated_at`,
		id, name, email, phone, company, status).Scan(&contact.ID, &contact.Name, &contact.Email, &contact.Phone,
		&contact.Company, &contact.Status, &contact.CreatedAt, &contact.UpdatedAt)
	if err != nil {
		return contact, err
	}
	if err := addSystemActivity(ctx, tx, contact.ID, nil, "Contact updated: "+contact.Name); err != nil {
		return contact, err
	}
	return contact, tx.Commit(ctx)
}

func (r *CRMRepository) DeleteContact(ctx context.Context, id string) (int64, int64, error) {
	var opportunities, activities int64
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM opportunities WHERE contact_id = $1`, id).Scan(&opportunities)
	if err != nil {
		return 0, 0, err
	}
	err = r.db.QueryRow(ctx, `SELECT count(*) FROM activities WHERE contact_id = $1 OR opportunity_id IN (SELECT id FROM opportunities WHERE contact_id = $1)`, id).Scan(&activities)
	if err != nil {
		return 0, 0, err
	}
	command, err := r.db.Exec(ctx, "DELETE FROM contacts WHERE id = $1", id)
	if err != nil {
		return 0, 0, err
	}
	if command.RowsAffected() == 0 {
		return 0, 0, pgx.ErrNoRows
	}
	return opportunities, activities, nil
}

func (r *CRMRepository) ListOpportunities(ctx context.Context, stage, contactID string) ([]model.Opportunity, error) {
	var stageArg, contactArg any
	if stage != "" {
		stageArg = stage
	}
	if contactID != "" {
		contactArg = contactID
	}
	return r.opportunities(ctx, `WHERE ($1::text IS NULL OR o.stage = $1) AND ($2::uuid IS NULL OR o.contact_id = $2::uuid)`, stageArg, contactArg)
}

func (r *CRMRepository) CreateOpportunity(ctx context.Context, opportunity model.Opportunity) (model.Opportunity, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return opportunity, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO opportunities (contact_id, title, value_rm, stage, position, closed_at)
		VALUES ($1, $2, $3::numeric, $4, $5, CASE WHEN $4 IN ('won','lost') THEN now() ELSE NULL END)
		RETURNING id::text, contact_id::text, title, value_rm::text, stage, position, closed_at, created_at, updated_at`,
		opportunity.ContactID, opportunity.Title, opportunity.ValueRM, opportunity.Stage, opportunity.Position).Scan(
		&opportunity.ID, &opportunity.ContactID, &opportunity.Title, &opportunity.ValueRM, &opportunity.Stage,
		&opportunity.Position, &opportunity.ClosedAt, &opportunity.CreatedAt, &opportunity.UpdatedAt)
	if err != nil {
		return opportunity, err
	}
	if err := addSystemActivity(ctx, tx, opportunity.ContactID, &opportunity.ID, "Opportunity created: "+opportunity.Title); err != nil {
		return opportunity, err
	}
	return opportunity, tx.Commit(ctx)
}

func (r *CRMRepository) UpdateOpportunity(ctx context.Context, id string, title, valueRM, stage *string, position *int) (model.Opportunity, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Opportunity{}, err
	}
	defer tx.Rollback(ctx)
	var result model.Opportunity
	err = tx.QueryRow(ctx, `UPDATE opportunities SET
		title = coalesce($2, title), value_rm = coalesce($3::numeric, value_rm),
		stage = coalesce($4, stage), position = coalesce($5, position),
		closed_at = CASE WHEN coalesce($4, stage) IN ('won','lost') THEN coalesce(closed_at, now()) ELSE NULL END,
		updated_at = now() WHERE id = $1
		RETURNING id::text, contact_id::text, title, value_rm::text, stage, position, closed_at, created_at, updated_at`,
		id, title, valueRM, stage, position).Scan(&result.ID, &result.ContactID, &result.Title, &result.ValueRM,
		&result.Stage, &result.Position, &result.ClosedAt, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return result, err
	}
	if err := addSystemActivity(ctx, tx, result.ContactID, &result.ID, "Opportunity updated: "+result.Title); err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}

func (r *CRMRepository) MoveOpportunity(ctx context.Context, id, stage string, position *int) (model.Opportunity, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.Opportunity{}, err
	}
	defer tx.Rollback(ctx)
	var result model.Opportunity
	var previousStage string
	err = tx.QueryRow(ctx, `SELECT contact_id::text, title, stage, position FROM opportunities WHERE id = $1 FOR UPDATE`, id).Scan(&result.ContactID, &result.Title, &previousStage, &result.Position)
	if err != nil {
		return result, err
	}
	if position == nil {
		position = &result.Position
	}
	err = tx.QueryRow(ctx, `UPDATE opportunities SET stage = $2, position = $3,
		closed_at = CASE WHEN $2 IN ('won','lost') THEN coalesce(closed_at, now()) ELSE NULL END,
		updated_at = now() WHERE id = $1
		RETURNING id::text, contact_id::text, title, value_rm::text, stage, position, closed_at, created_at, updated_at`,
		id, stage, *position).Scan(&result.ID, &result.ContactID, &result.Title, &result.ValueRM, &result.Stage,
		&result.Position, &result.ClosedAt, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return result, err
	}
	if previousStage != stage {
		if err := addSystemActivity(ctx, tx, result.ContactID, &result.ID, fmt.Sprintf("Opportunity moved from %s to %s: %s", previousStage, stage, result.Title)); err != nil {
			return result, err
		}
	}
	return result, tx.Commit(ctx)
}

func (r *CRMRepository) DeleteOpportunity(ctx context.Context, id string) error {
	command, err := r.db.Exec(ctx, "DELETE FROM opportunities WHERE id = $1", id)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *CRMRepository) ListActivities(ctx context.Context, contactID, opportunityID string) ([]model.Activity, error) {
	var contactArg, opportunityArg any
	if contactID != "" {
		contactArg = contactID
	}
	if opportunityID != "" {
		opportunityArg = opportunityID
	}
	return r.activities(ctx, `WHERE ($1::uuid IS NULL OR a.contact_id = $1::uuid) AND ($2::uuid IS NULL OR a.opportunity_id = $2::uuid)`, contactArg, opportunityArg)
}

func (r *CRMRepository) CreateActivity(ctx context.Context, activity model.Activity) (model.Activity, error) {
	var contactArg any
	if activity.ContactID != nil {
		contactArg = *activity.ContactID
	}
	var opportunityArg any
	if activity.OpportunityID != nil {
		opportunityArg = *activity.OpportunityID
	}
	err := r.db.QueryRow(ctx, `INSERT INTO activities (type, contact_id, opportunity_id, occurred_at, description, source)
		SELECT $1, coalesce($2::uuid, o.contact_id), $3::uuid, $4, $5, 'manual'
		FROM (SELECT 1) AS one LEFT JOIN opportunities o ON o.id = $3::uuid
		WHERE ($2::uuid IS NOT NULL OR o.id IS NOT NULL)
		AND ($2::uuid IS NULL OR o.contact_id IS NULL OR o.contact_id = $2::uuid)
		RETURNING id::text, type, contact_id::text, opportunity_id::text, occurred_at, description, source, created_at`,
		activity.Type, contactArg, opportunityArg, activity.OccurredAt, activity.Description).Scan(
		&activity.ID, &activity.Type, &activity.ContactID, &activity.OpportunityID, &activity.OccurredAt,
		&activity.Description, &activity.Source, &activity.CreatedAt)
	return activity, err
}

func (r *CRMRepository) Dashboard(ctx context.Context) (model.Dashboard, error) {
	result := model.Dashboard{OpenValueRM: "0.00", WonValueRM: "0.00", RecentActivities: []model.Activity{}}
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM contacts").Scan(&result.ContactCount); err != nil {
		return result, err
	}
	if err := r.db.QueryRow(ctx, `SELECT count(*) FILTER (WHERE stage NOT IN ('won','lost')),
		coalesce(sum(value_rm) FILTER (WHERE stage NOT IN ('won','lost')), 0.00)::text,
		coalesce(sum(value_rm) FILTER (WHERE stage = 'won'), 0.00)::text FROM opportunities`).Scan(
		&result.OpenCount, &result.OpenValueRM, &result.WonValueRM); err != nil {
		return result, err
	}
	activities, err := r.activities(ctx, "", "")
	if err != nil {
		return result, err
	}
	if len(activities) > 10 {
		activities = activities[:10]
	}
	result.RecentActivities = activities
	return result, nil
}

func (r *CRMRepository) opportunities(ctx context.Context, clause string, args ...any) ([]model.Opportunity, error) {
	query := `SELECT o.id::text, o.contact_id::text, o.title, o.value_rm::text, o.stage, o.position, o.closed_at, o.created_at, o.updated_at,
		c.id::text, c.name, c.email, c.phone, c.company, c.status, c.created_at, c.updated_at
		FROM opportunities o JOIN contacts c ON c.id = o.contact_id ` + clause + `
		ORDER BY CASE o.stage WHEN 'new' THEN 1 WHEN 'contacted' THEN 2 WHEN 'qualified' THEN 3 WHEN 'proposal' THEN 4 WHEN 'won' THEN 5 ELSE 6 END,
		         o.position, o.created_at DESC`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Opportunity
	for rows.Next() {
		var item model.Opportunity
		var contact model.Contact
		if err := rows.Scan(&item.ID, &item.ContactID, &item.Title, &item.ValueRM, &item.Stage, &item.Position, &item.ClosedAt, &item.CreatedAt, &item.UpdatedAt,
			&contact.ID, &contact.Name, &contact.Email, &contact.Phone, &contact.Company, &contact.Status, &contact.CreatedAt, &contact.UpdatedAt); err != nil {
			return nil, err
		}
		item.Contact = &contact
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *CRMRepository) activities(ctx context.Context, clause string, args ...any) ([]model.Activity, error) {
	limit := ""
	if clause == "" {
		limit = " LIMIT 10"
	}
	query := `SELECT a.id::text, a.type, a.contact_id::text, a.opportunity_id::text, a.occurred_at, a.description, a.source, a.created_at,
		c.name, o.title FROM activities a LEFT JOIN contacts c ON c.id = a.contact_id
		LEFT JOIN opportunities o ON o.id = a.opportunity_id ` + clause + ` ORDER BY a.occurred_at DESC` + limit
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.Activity
	for rows.Next() {
		var item model.Activity
		if err := rows.Scan(&item.ID, &item.Type, &item.ContactID, &item.OpportunityID, &item.OccurredAt, &item.Description,
			&item.Source, &item.CreatedAt, &item.ContactName, &item.OpportunityTitle); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func addSystemActivity(ctx context.Context, tx pgx.Tx, contactID string, opportunityID *string, description string) error {
	var opportunityArg any
	if opportunityID != nil {
		opportunityArg = *opportunityID
	}
	_, err := tx.Exec(ctx, `INSERT INTO activities (type, contact_id, opportunity_id, occurred_at, description, source)
		VALUES ('note', $1, $2, now(), $3, 'system')`, contactID, opportunityArg, description)
	return err
}

func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
func IsDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
func IsInvalidDatabaseValue(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "22P02" || pgErr.Code == "23503" || pgErr.Code == "23514" || pgErr.Code == "22003")
}

func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
