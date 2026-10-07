package journal

type Repository interface {
	Create(entry Entry) (Entry, error)
	ListByUser(userID string) ([]Entry, error)
}
