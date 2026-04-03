package verifyhash

import "testing"

func TestBuildRawDiplomaString(t *testing.T) {
	t.Parallel()

	input := DiplomaHashInput{
		FullName:      "Ivanov Ivan Ivanovich",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}

	raw, err := BuildRawDiplomaString(input)
	if err != nil {
		t.Fatalf("build raw diploma string: %v", err)
	}

	const expected = "Ivanov Ivan Ivanovich|DVS-2024-001234|Software Engineering|2024|550e8400-e29b-41d4-a716-446655440000|abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	if raw != expected {
		t.Fatalf("unexpected raw string: got %q want %q", raw, expected)
	}
}

func TestHashDiplomaInputExpectedValue(t *testing.T) {
	t.Parallel()

	hash, err := HashDiplomaInput(DiplomaHashInput{
		FullName:      "Ivanov Ivan Ivanovich",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	})
	if err != nil {
		t.Fatalf("hash diploma input: %v", err)
	}

	const expected = "ad48ff40e10da83a32fcf59b1e4cc2db3ec06273238d4c4e3b693c86e901e875"
	if hash != expected {
		t.Fatalf("unexpected hash: got %s want %s", hash, expected)
	}
}
