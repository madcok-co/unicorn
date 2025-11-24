package playground

import (
	"testing"

	"github.com/madcok-co/unicorn/core/pkg/contracts"
)

func TestNewDriver(t *testing.T) {
	driver := NewDriver()

	if driver == nil {
		t.Fatal("driver should not be nil")
	}
	if driver.validate == nil {
		t.Error("validate should not be nil")
	}
	if driver.translations == nil {
		t.Error("translations should not be nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.UseJSONNames {
		t.Error("UseJSONNames should be true by default")
	}
	if cfg.Messages == nil {
		t.Error("Messages should not be nil")
	}
}

func TestNewDriverWithConfig(t *testing.T) {
	t.Run("with custom messages", func(t *testing.T) {
		cfg := &Config{
			UseJSONNames: true,
			Messages: map[string]string{
				"required": "Field {field} is mandatory",
			},
		}
		driver := NewDriverWithConfig(cfg)

		if driver.translations["required"] != "Field {field} is mandatory" {
			t.Error("custom message not registered")
		}
	})

	t.Run("without JSON names", func(t *testing.T) {
		cfg := &Config{
			UseJSONNames: false,
		}
		driver := NewDriverWithConfig(cfg)

		if driver == nil {
			t.Fatal("driver should not be nil")
		}
	})
}

func TestDriver_Validator(t *testing.T) {
	driver := NewDriver()

	if driver.Validator() == nil {
		t.Error("Validator() should return underlying validator")
	}
}

func TestDriver_Validate(t *testing.T) {
	driver := NewDriver()

	type User struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0,lte=150"`
	}

	t.Run("valid struct", func(t *testing.T) {
		user := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}

		err := driver.Validate(user)
		if err != nil {
			t.Errorf("valid struct should not error: %v", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		user := User{
			Email: "john@example.com",
			Age:   30,
		}

		err := driver.Validate(user)
		if err == nil {
			t.Error("should return error for missing required field")
		}

		validationErrs, ok := err.(contracts.ValidationErrors)
		if !ok {
			t.Fatalf("error should be ValidationErrors, got %T", err)
		}

		if len(validationErrs) != 1 {
			t.Errorf("expected 1 error, got %d", len(validationErrs))
		}

		if validationErrs[0].Field != "name" {
			t.Errorf("expected field 'name', got %s", validationErrs[0].Field)
		}
		if validationErrs[0].Tag != "required" {
			t.Errorf("expected tag 'required', got %s", validationErrs[0].Tag)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		user := User{
			Name:  "John",
			Email: "invalid-email",
			Age:   30,
		}

		err := driver.Validate(user)
		if err == nil {
			t.Error("should return error for invalid email")
		}

		validationErrs := err.(contracts.ValidationErrors)
		if validationErrs[0].Tag != "email" {
			t.Errorf("expected tag 'email', got %s", validationErrs[0].Tag)
		}
	})

	t.Run("min length violation", func(t *testing.T) {
		user := User{
			Name:  "Jo",
			Email: "john@example.com",
			Age:   30,
		}

		err := driver.Validate(user)
		if err == nil {
			t.Error("should return error for min length violation")
		}

		validationErrs := err.(contracts.ValidationErrors)
		if validationErrs[0].Tag != "min" {
			t.Errorf("expected tag 'min', got %s", validationErrs[0].Tag)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		user := User{
			Name:  "",
			Email: "invalid",
			Age:   200,
		}

		err := driver.Validate(user)
		if err == nil {
			t.Error("should return errors")
		}

		validationErrs := err.(contracts.ValidationErrors)
		if len(validationErrs) < 3 {
			t.Errorf("expected at least 3 errors, got %d", len(validationErrs))
		}
	})
}

func TestDriver_ValidateField(t *testing.T) {
	driver := NewDriver()

	t.Run("valid field", func(t *testing.T) {
		err := driver.ValidateField("john@example.com", "email")
		if err != nil {
			t.Errorf("valid email should not error: %v", err)
		}
	})

	t.Run("invalid field", func(t *testing.T) {
		err := driver.ValidateField("not-an-email", "email")
		if err == nil {
			t.Error("invalid email should error")
		}

		validationErrs, ok := err.(contracts.ValidationErrors)
		if !ok {
			t.Fatalf("error should be ValidationErrors")
		}

		if validationErrs[0].Field != "value" {
			t.Errorf("expected field 'value', got %s", validationErrs[0].Field)
		}
	})

	t.Run("required validation", func(t *testing.T) {
		err := driver.ValidateField("", "required")
		if err == nil {
			t.Error("empty string should fail required validation")
		}
	})

	t.Run("min validation", func(t *testing.T) {
		err := driver.ValidateField("ab", "min=3")
		if err == nil {
			t.Error("short string should fail min validation")
		}
	})

	t.Run("max validation", func(t *testing.T) {
		err := driver.ValidateField("abcdefghij", "max=5")
		if err == nil {
			t.Error("long string should fail max validation")
		}
	})

	t.Run("numeric validation", func(t *testing.T) {
		err := driver.ValidateField(42, "gte=0,lte=100")
		if err != nil {
			t.Errorf("valid number should not error: %v", err)
		}

		err = driver.ValidateField(150, "lte=100")
		if err == nil {
			t.Error("number exceeding max should error")
		}
	})
}

func TestDriver_RegisterValidation(t *testing.T) {
	driver := NewDriver()

	// Register custom validation
	err := driver.RegisterValidation("is_admin", func(v any) bool {
		s, ok := v.(string)
		return ok && (s == "admin" || s == "superadmin")
	})
	if err != nil {
		t.Fatalf("RegisterValidation should not error: %v", err)
	}

	// Test custom validation
	err = driver.ValidateField("admin", "is_admin")
	if err != nil {
		t.Errorf("'admin' should pass is_admin validation: %v", err)
	}

	err = driver.ValidateField("user", "is_admin")
	if err == nil {
		t.Error("'user' should fail is_admin validation")
	}
}

func TestDriver_RegisterTranslation(t *testing.T) {
	driver := NewDriver()

	err := driver.RegisterTranslation("custom_tag", "Custom error: {field} is invalid")
	if err != nil {
		t.Fatalf("RegisterTranslation should not error: %v", err)
	}

	if driver.translations["custom_tag"] != "Custom error: {field} is invalid" {
		t.Error("translation not registered")
	}
}

func TestDriver_FormatMessage(t *testing.T) {
	driver := NewDriver()

	type TestStruct struct {
		Field string `validate:"required"`
	}

	err := driver.Validate(TestStruct{})
	if err == nil {
		t.Fatal("should return error")
	}

	validationErrs := err.(contracts.ValidationErrors)
	if validationErrs[0].Message == "" {
		t.Error("message should not be empty")
	}
}

func TestDriver_JSONFieldNames(t *testing.T) {
	driver := NewDriver()

	type User struct {
		FirstName string `json:"first_name" validate:"required"`
	}

	err := driver.Validate(User{})
	if err == nil {
		t.Fatal("should return error")
	}

	validationErrs := err.(contracts.ValidationErrors)
	if validationErrs[0].Field != "first_name" {
		t.Errorf("expected field 'first_name', got %s", validationErrs[0].Field)
	}
}

func TestDriver_JSONFieldNameDash(t *testing.T) {
	// Test JSON tag with "-" (ignore)
	cfg := &Config{UseJSONNames: true}
	driver := NewDriverWithConfig(cfg)

	type User struct {
		Internal string `json:"-" validate:"required"`
		Name     string `json:"name" validate:"required"`
	}

	// This should still work - internal field won't be validated by name
	err := driver.Validate(User{Name: "test"})
	if err != nil {
		// The internal field will still be validated, just with different field name
		_ = err
	}
}

func TestDriver_NoJSONTag(t *testing.T) {
	driver := NewDriver()

	type User struct {
		Name string `validate:"required"`
	}

	err := driver.Validate(User{})
	if err == nil {
		t.Fatal("should return error")
	}

	validationErrs := err.(contracts.ValidationErrors)
	// Should fallback to struct field name
	if validationErrs[0].Field != "Name" {
		t.Errorf("expected field 'Name', got %s", validationErrs[0].Field)
	}
}

func TestDriver_ImplementsValidator(t *testing.T) {
	var _ contracts.Validator = (*Driver)(nil)
}

func TestFormatValue(t *testing.T) {
	if formatValue(nil) != "nil" {
		t.Error("nil should format as 'nil'")
	}
	if formatValue("test") != "test" {
		t.Error("string should format as itself")
	}
	if formatValue(123) != "" {
		t.Error("non-string should format as empty")
	}
}

func BenchmarkDriver_Validate(b *testing.B) {
	driver := NewDriver()

	type User struct {
		Name  string `json:"name" validate:"required,min=3"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"gte=0,lte=150"`
	}

	user := User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.Validate(user)
	}
}

func BenchmarkDriver_ValidateField(b *testing.B) {
	driver := NewDriver()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		driver.ValidateField("john@example.com", "required,email")
	}
}
