package conversation

import "testing"

func TestValidUUIDAceeptsCanonicalID(t *testing.T) {
	if !validUUID("e26d9ad3-94bf-40cf-9264-04a09682f4ee") {
		t.Fatal("valid canonical UUID rejected")
	}
	if validUUID("e26d9ad3-94bf-40cf-9264-aaaa-04a09682f4ee") {
		t.Fatal("six group UUID erroneously accepted")
	}
	if validUUID("not-an-id") {
		t.Fatal("malformed UUID erroneously accepted")
	}
}

func TestDeletionConfirmationValue(t *testing.T) {
	if DeleteConversationConfirmation != "DELETE CONVERSATION" {
		t.Fatal("deletion confirmation changed unexpectedly")
	}
}
