package validator

import (
	"strings"
	"testing"
)

type TestUser struct {
	Name  string `validate:"required,min=2,max=50"`
	Email string `validate:"required,email"`
	Age   int    `validate:"required,min=18,max=120"`
}

type TestProduct struct {
	Name  string  `validate:"required"`
	Price float64 `validate:"required,min=0"`
	SKU   string  `validate:"required,len=8"`
}

func TestSimpleValidator(t *testing.T) {
	v := NewSimpleValidator()

	t.Run("Valid struct", func(t *testing.T) {
		user := TestUser{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   25,
		}

		err := v.Validate(user)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("Missing required field", func(t *testing.T) {
		user := TestUser{
			Name: "John",
			Age:  25,
		}

		err := v.Validate(user)
		if err == nil {
			t.Error("expected error for missing email")
		}

		verrs, ok := err.(ValidationErrors)
		if !ok {
			t.Fatalf("expected ValidationErrors, got %T", err)
		}

		hasEmailError := false
		for _, e := range verrs {
			if e.Field == "Email" && e.Tag == "required" {
				hasEmailError = true
				break
			}
		}
		if !hasEmailError {
			t.Error("expected email required error")
		}
	})

	t.Run("Invalid email format", func(t *testing.T) {
		user := TestUser{
			Name:  "John",
			Email: "invalid-email",
			Age:   25,
		}

		err := v.Validate(user)
		if err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("Min validation", func(t *testing.T) {
		user := TestUser{
			Name:  "J", // min 2 characters
			Email: "john@example.com",
			Age:   25,
		}

		err := v.Validate(user)
		if err == nil {
			t.Error("expected error for name too short")
		}
	})

	t.Run("Max validation", func(t *testing.T) {
		user := TestUser{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   150, // max 120
		}

		err := v.Validate(user)
		if err == nil {
			t.Error("expected error for age too high")
		}
	})

	t.Run("Len validation", func(t *testing.T) {
		product := TestProduct{
			Name:  "Widget",
			Price: 9.99,
			SKU:   "ABC", // should be 8 characters
		}

		err := v.Validate(product)
		if err == nil {
			t.Error("expected error for SKU wrong length")
		}
	})

	t.Run("Valid product", func(t *testing.T) {
		product := TestProduct{
			Name:  "Widget",
			Price: 9.99,
			SKU:   "ABCD1234", // exactly 8 characters
		}

		err := v.Validate(product)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestValidationErrors(t *testing.T) {
	errs := ValidationErrors{
		{Field: "Name", Tag: "required", Message: "Name is required"},
		{Field: "Email", Tag: "email", Message: "Invalid email format"},
		{Field: "Name", Tag: "min", Message: "Name too short"},
	}

	t.Run("Error string", func(t *testing.T) {
		// Error() returns all errors joined with semicolons
		errStr := errs.Error()
		if !strings.Contains(errStr, "Name is required") {
			t.Errorf("expected error to contain 'Name is required', got: %s", errStr)
		}
	})

	t.Run("HasErrors", func(t *testing.T) {
		if !errs.HasErrors() {
			t.Error("expected HasErrors to be true")
		}

		empty := ValidationErrors{}
		if empty.HasErrors() {
			t.Error("expected HasErrors to be false for empty")
		}
	})

	t.Run("ByField", func(t *testing.T) {
		nameErrors := errs.ByField("Name")
		if len(nameErrors) != 2 {
			t.Errorf("expected 2 name errors, got %d", len(nameErrors))
		}
	})

	t.Run("ToMap", func(t *testing.T) {
		m := errs.ToMap()
		if m["Name"] == "" {
			t.Error("expected name error in map")
		}
		if m["Email"] == "" {
			t.Error("expected email error in map")
		}
	})
}

func TestCustomRule(t *testing.T) {
	v := NewSimpleValidator()

	// Register custom rule
	v.RegisterRule("even", func(value any, param string) bool {
		if i, ok := value.(int); ok {
			return i%2 == 0
		}
		return false
	})

	type EvenNumber struct {
		Value int `validate:"even"`
	}

	t.Run("Valid custom rule", func(t *testing.T) {
		obj := EvenNumber{Value: 4}
		err := v.Validate(obj)
		if err != nil {
			t.Errorf("expected no error: %v", err)
		}
	})

	t.Run("Invalid custom rule", func(t *testing.T) {
		obj := EvenNumber{Value: 3}
		err := v.Validate(obj)
		if err == nil {
			t.Error("expected error for odd number")
		}
	})
}
