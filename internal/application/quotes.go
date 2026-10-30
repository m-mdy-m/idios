package application

import (
	"context"
	"idios/internal/domain"
	"idios/internal/ports"
	"idios/internal/validation"
	"strings"
)

type QuotesService struct{ store ports.QuotesStore }

func (s *QuotesService) ListQuotes(ctx context.Context) ([]domain.Quote, error) {
	return s.store.ListQuotes(ctx)
}

func (s *QuotesService) CreateQuote(ctx context.Context, quote, author string) (domain.Quote, error) {
	if err := invalidInput(validation.Text("quote text", quote, true, 10000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote author", author, false, 500)); err != nil {
		return domain.Quote{}, err
	}
	return s.store.CreateQuote(ctx, strings.TrimSpace(quote), strings.TrimSpace(author))
}

func (s *QuotesService) CreateQuoteWithSource(ctx context.Context, quote domain.Quote) (domain.Quote, error) {
	if err := invalidInput(validation.Text("quote text", quote.Text, true, 10000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote author", quote.Author, false, 500)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote source", quote.Source, false, 1000)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.URL("quote source URL", quote.SourceURL)); err != nil {
		return domain.Quote{}, err
	}
	if err := invalidInput(validation.Text("quote category", quote.Category, false, 100)); err != nil {
		return domain.Quote{}, err
	}
	return s.store.CreateQuoteWithSource(ctx, quote)
}

func (s *QuotesService) DeleteQuote(ctx context.Context, id string) error {
	if err := invalidInput(validation.Text("quote reference", id, true, 256)); err != nil {
		return err
	}
	return s.store.DeleteQuote(ctx, strings.TrimSpace(id))
}

func (s *QuotesService) NextQuote(ctx context.Context) (domain.Quote, error) {
	return s.store.NextQuote(ctx)
}
