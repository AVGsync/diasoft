package verifyhash

import "testing"

func TestBuildRawDiplomaString(t *testing.T) {
	t.Parallel()

	input := DiplomaHashInput{
		FullName:      "Ivanov Ivan Ivanovich",
		DiplomaNumber: "DVS-2024-001234",
		Specialty:     "Software Engineering",
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}

	raw, err := BuildRawDiplomaString(input)
	if err != nil {
		t.Fatalf("build raw diploma string: %v", err)
	}

	const expected = "Ivanov Ivan Ivanovich|DVS-2024-001234|Software Engineering|Bachelor|FKN|2024|550e8400-e29b-41d4-a716-446655440000|abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
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
		Degree:        "Bachelor",
		Faculty:       "FKN",
		Year:          2024,
		VUZID:         "550e8400-e29b-41d4-a716-446655440000",
		Salt:          "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	})
	if err != nil {
		t.Fatalf("hash diploma input: %v", err)
	}

	const expected = "4ea7b954dc605d049670a027e5357d4a9c2892f2ea6da1490cecd4e387515860"
	if hash != expected {
		t.Fatalf("unexpected hash: got %s want %s", hash, expected)
	}
}
