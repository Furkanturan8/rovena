package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Furkanturan8/rovena/services/catalog/internal/domain"
	"github.com/Furkanturan8/rovena/services/catalog/internal/repository"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// priceFromExpr is the cheapest active variant price of the product.
const priceFromExpr = "(SELECT MIN(pv.price_cents) FROM variants pv WHERE pv.product_id = p.id AND pv.active)"

const maxSearchWords = 5

type ProductRepository struct {
	db *bun.DB
}

func NewProductRepository(db *bun.DB) *ProductRepository { return &ProductRepository{db: db} }

var _ repository.ProductRepository = (*ProductRepository)(nil)

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(productRowFrom(p)).Exec(ctx); err != nil {
			return err
		}
		if err := insertImages(ctx, tx, p.ID, p.Images); err != nil {
			return err
		}
		if len(p.Variants) == 0 {
			return nil
		}
		rows := make([]variantRow, len(p.Variants))
		for i := range p.Variants {
			rows[i] = *variantRowFrom(&p.Variants[i])
		}
		_, err := tx.NewInsert().Model(&rows).Exec(ctx)
		return err
	})
	if err != nil {
		return mapMissingCategory(err)
	}
	return nil
}

func insertImages(ctx context.Context, tx bun.Tx, productID uuid.UUID, images []domain.Image) error {
	if len(images) == 0 {
		return nil
	}
	rows := make([]imageRow, len(images))
	for i, img := range images {
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		rows[i] = imageRow{ID: id, ProductID: productID, URL: img.URL, Alt: img.Alt, Position: i}
	}
	_, err := tx.NewInsert().Model(&rows).Exec(ctx)
	return err
}

func (r *ProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Product, error) {
	return r.getOne(ctx, "p.id = ?", id)
}

func (r *ProductRepository) GetBySlug(ctx context.Context, slug string) (*domain.Product, error) {
	return r.getOne(ctx, "p.slug = ?", slug)
}

func (r *ProductRepository) getOne(ctx context.Context, where string, arg any) (*domain.Product, error) {
	row := new(productRow)
	err := r.db.NewSelect().Model(row).Where(where, arg).Limit(1).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	p := row.toDomain()

	var images []imageRow
	if err := r.db.NewSelect().Model(&images).Where("i.product_id = ?", p.ID).OrderExpr("i.position").Scan(ctx); err != nil {
		return nil, err
	}
	for _, img := range images {
		p.Images = append(p.Images, domain.Image{URL: img.URL, Alt: img.Alt, Position: img.Position})
	}

	var variants []variantRow
	if err := r.db.NewSelect().Model(&variants).Where("v.product_id = ?", p.ID).OrderExpr("v.created_at, v.id").Scan(ctx); err != nil {
		return nil, err
	}
	for i := range variants {
		p.Variants = append(p.Variants, *variants[i].toDomain())
	}
	return p, nil
}

func (r *ProductRepository) List(ctx context.Context, f domain.ProductFilter) ([]domain.ProductSummary, int, error) {
	var rows []summaryRow
	q := r.db.NewSelect().Model(&rows).
		Column("p.id", "p.category_id", "p.name", "p.slug", "p.brand", "p.status").
		ColumnExpr(priceFromExpr + " AS price_from")

	if f.Status != nil {
		q = q.Where("p.status = ?", string(*f.Status))
	}
	// Every word must appear in the accent-folded search_text (AND semantics).
	words := domain.SearchWords(f.Query)
	for _, w := range words[:min(len(words), maxSearchWords)] {
		q = q.Where("p.search_text LIKE ?", "%"+w+"%")
	}
	if f.CategorySlug != "" {
		q = q.Where(`p.category_id IN (
			WITH RECURSIVE tree AS (
				SELECT id FROM categories WHERE slug = ?
				UNION ALL
				SELECT c.id FROM categories c JOIN tree t ON c.parent_id = t.id
			) SELECT id FROM tree)`, f.CategorySlug)
	}

	// All variant-level filters must match the same variant (e.g. size M in black).
	conds := []string{"v.product_id = p.id", "v.active"}
	var args []any
	if f.Size != "" {
		conds = append(conds, "lower(v.size) = lower(?)")
		args = append(args, f.Size)
	}
	if f.Color != "" {
		conds = append(conds, "lower(v.color) = lower(?)")
		args = append(args, f.Color)
	}
	if f.MinPrice != nil {
		conds = append(conds, "v.price_cents >= ?")
		args = append(args, *f.MinPrice)
	}
	if f.MaxPrice != nil {
		conds = append(conds, "v.price_cents <= ?")
		args = append(args, *f.MaxPrice)
	}
	if len(args) > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM variants v WHERE "+strings.Join(conds, " AND ")+")", args...)
	}

	switch f.Sort {
	case domain.SortPriceAsc:
		q = q.OrderExpr(priceFromExpr + " ASC NULLS LAST, p.id")
	case domain.SortPriceDesc:
		q = q.OrderExpr(priceFromExpr + " DESC NULLS LAST, p.id")
	case domain.SortName:
		q = q.OrderExpr("p.name ASC, p.id")
	default:
		q = q.OrderExpr("p.created_at DESC, p.id DESC")
	}

	total, err := q.Limit(int64(f.PageSize)).Offset(int64((f.Page - 1) * f.PageSize)).ScanAndCount(ctx)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	thumbs, err := r.thumbnails(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	out := make([]domain.ProductSummary, len(rows))
	for i, row := range rows {
		out[i] = domain.ProductSummary{
			ID: row.ID, CategoryID: row.CategoryID, Name: row.Name, Slug: row.Slug, Brand: row.Brand,
			Status: domain.ProductStatus(row.Status), PriceFromCents: row.PriceFrom,
			Currency: domain.DefaultCurrency, ThumbnailURL: thumbs[row.ID],
		}
	}
	return out, int(total), nil
}

func (r *ProductRepository) thumbnails(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []imageRow
	err := r.db.NewSelect().Model(&rows).
		DistinctOn("i.product_id").
		Where("i.product_id IN (?)", bun.In(ids)).
		OrderExpr("i.product_id, i.position").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProductID] = row.URL
	}
	return out, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *domain.Product, replaceImages bool) error {
	p.UpdatedAt = time.Now()
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := lockProduct(ctx, tx, p.ID); err != nil {
			return err
		}
		if p.Status == domain.StatusActive {
			active, err := tx.NewSelect().Model((*variantRow)(nil)).
				Where("v.product_id = ?", p.ID).Where("v.active").Count(ctx)
			if err != nil {
				return err
			}
			if active == 0 {
				return domain.ErrNoActiveVariant
			}
		}
		if _, err := tx.NewUpdate().Model((*productRow)(nil)).
			Set("category_id = ?", p.CategoryID).
			Set("name = ?", p.Name).
			Set("description = ?", p.Description).
			Set("brand = ?", p.Brand).
			Set("status = ?", string(p.Status)).
			Set("search_text = ?", domain.SearchText(p.Name, p.Brand)).
			Set("updated_at = ?", p.UpdatedAt).
			Where("id = ?", p.ID).
			Exec(ctx); err != nil {
			return err
		}
		if !replaceImages {
			return nil
		}
		if _, err := tx.NewDelete().Model((*imageRow)(nil)).Where("product_id = ?", p.ID).Exec(ctx); err != nil {
			return err
		}
		return insertImages(ctx, tx, p.ID, p.Images)
	})
	if errors.Is(err, domain.ErrProductNotFound) || errors.Is(err, domain.ErrNoActiveVariant) {
		return err
	}
	if err != nil {
		return mapMissingCategory(err)
	}
	return nil
}

func (r *ProductRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	return r.db.NewSelect().Model((*productRow)(nil)).Where("p.slug = ?", slug).Exists(ctx)
}
