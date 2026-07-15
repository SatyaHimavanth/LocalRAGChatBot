package app

import "changeme/internal/store"

// GetCollectionInsights returns richer analytics for each collection.
func (s *ChatService) GetCollectionInsights() ([]store.CollectionInsight, error) {
	if s.DB == nil {
		return nil, nil
	}
	return store.GetCollectionInsights(s.DB)
}
