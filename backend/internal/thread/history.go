package thread

import (
	"errors"
	"journalapp/internal/capsule"
	"sort"

	"strings"
)

var (
	ErrHistoryInconsistent = errors.New(
		"capsule history contains an invalid relationship",
	)
	ErrHistoryTooLarge = errors.New(
		"capsule history exceeds the retrieval limit",
	)
)

const MaxHistoryCapsules = 1024

type HistoryNode struct {
	Capsule     capsule.CapsuleMetadata `json:"capsule"`
	Reflections []HistoryReflection     `json:"reflections"`
}

type HistoryReflection struct {
	Reflection
	Child *HistoryNode `json:"child,omitempty"`
}

func (s *Service) GetHistory(
	userID string,
	capsuleID string,
) (HistoryNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID = strings.TrimSpace(userID)
	capsuleID = strings.TrimSpace(capsuleID)

	root, err := s.capsules.GetMetadata(userID, capsuleID)

	if err != nil {
		return HistoryNode{}, err
	}

	visited := make(map[string]struct{})

	return s.buildHistory(userID, root, visited)

}

func (s *Service) buildHistory(
	userID string,
	metadata capsule.CapsuleMetadata,
	visited map[string]struct{},
) (HistoryNode, error) {
	if _, exists := visited[metadata.ID]; exists {
		return HistoryNode{}, ErrHistoryInconsistent
	}

	if len(visited) >= MaxHistoryCapsules {
		return HistoryNode{}, ErrHistoryTooLarge
	}

	visited[metadata.ID] = struct{}{}

	node := HistoryNode{
		Capsule:     metadata,
		Reflections: make([]HistoryReflection, 0),
	}

	if metadata.Status != capsule.StatusReturned {
		return node, nil
	}

	reflections, err := s.repository.ListByCapsule(
		userID,
		metadata.ID,
	)

	if err != nil {
		return HistoryNode{}, err
	}

	sort.Slice(reflections, func(i, j int) bool {
		if reflections[i].CreatedAt.Equal(
			reflections[j].CreatedAt,
		) {
			return reflections[i].ID < reflections[j].ID
		}

		return reflections[i].CreatedAt.Before(
			reflections[j].CreatedAt,
		)
	})

	for _, reflection := range reflections {
		if reflection.UserID != userID ||
			reflection.CapsuleID != metadata.ID {
			return HistoryNode{}, ErrHistoryInconsistent
		}

		item := HistoryReflection{
			Reflection: reflection,
		}

		switch reflection.Status {
		case StatusDraft:
			if reflection.SealedCapsuleID != "" {
				return HistoryNode{}, ErrHistoryInconsistent
			}

		case StatusSealed:
			if reflection.SealedCapsuleID == "" {
				return HistoryNode{}, ErrHistoryInconsistent
			}

			item.Reflection.Body = ""

			childMeta, err := s.capsules.GetMetadata(
				userID,
				reflection.SealedCapsuleID,
			)

			if err != nil {
				return HistoryNode{}, ErrHistoryInconsistent
			}

			if childMeta.ParentCapsuleID != metadata.ID ||
				childMeta.SourceReflectionID != reflection.ID {
				return HistoryNode{}, ErrHistoryInconsistent
			}

			child, err := s.buildHistory(
				userID,
				childMeta,
				visited,
			)

			if err != nil {
				return HistoryNode{}, err
			}

			item.Child = &child

		default:
			return HistoryNode{}, ErrHistoryInconsistent

		}

		node.Reflections = append(node.Reflections, item)

	}

	return node, nil

}
