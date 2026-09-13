package handlers

import (
	"context"

	"bookreviews/internal/models"
)

func (h *Handler) afterReviewChange(ctx context.Context, bookIDs ...int64) {
	if err := h.stats.InvalidateReview(ctx, bookIDs...); err != nil {
		h.logger.Error("invalidar cache de reseña", "error", err)
	}
}

func (h *Handler) afterReviewUpsert(ctx context.Context, review models.Review, extraBookIDs ...int64) {
	ids := append([]int64{review.BookID}, extraBookIDs...)
	h.afterReviewChange(ctx, ids...)
	if err := h.searcher.IndexReview(ctx, review); err != nil {
		h.logger.Error("indexar reseña", "id", review.ID, "error", err)
	}
}

func (h *Handler) afterReviewRemoved(ctx context.Context, review models.Review) {
	h.afterReviewChange(ctx, review.BookID)
	if err := h.searcher.DeleteReview(ctx, review.ID); err != nil {
		h.logger.Error("borrar reseña del índice", "id", review.ID, "error", err)
	}
}

func (h *Handler) afterSaleChange(ctx context.Context) {
	if err := h.stats.InvalidateSale(ctx); err != nil {
		h.logger.Error("invalidar cache de venta", "error", err)
	}
}

func (h *Handler) afterCatalogChange(ctx context.Context) {
	if err := h.stats.InvalidateCatalog(ctx); err != nil {
		h.logger.Error("invalidar cache de catálogo", "error", err)
	}
}

func (h *Handler) afterBookUpsert(ctx context.Context, book models.Book) {
	h.afterCatalogChange(ctx)
	if err := h.searcher.IndexBook(ctx, book); err != nil {
		h.logger.Error("indexar libro", "id", book.ID, "error", err)
	}
}

func (h *Handler) afterBookRemoved(ctx context.Context, bookIDs ...int64) {
	if err := h.stats.InvalidateBookRemoved(ctx, bookIDs...); err != nil {
		h.logger.Error("invalidar cache de libro borrado", "error", err)
	}
	for _, id := range bookIDs {
		if err := h.searcher.DeleteBook(ctx, id); err != nil {
			h.logger.Error("borrar libro del índice", "id", id, "error", err)
		}
	}
}
