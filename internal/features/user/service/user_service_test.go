package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"jungo/internal/features/user/domain"
)

// fakeUserRepository is a hand-written test double for domain.UserRepository.
// Each Func field defaults to nil; a test that hits an unset one will panic,
// which surfaces unintended calls immediately instead of silently zero-valuing them.
type fakeUserRepository struct {
	CreateFunc     func(ctx context.Context, input domain.CreateUserInput, passwordHash string) (*domain.User, error)
	GetByUUIDFunc  func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmailFunc func(ctx context.Context, email string) (*domain.User, error)
	ListFunc       func(ctx context.Context, filter domain.UserListFilter) ([]*domain.User, int64, error)
	UpdateFunc     func(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error)
	DeleteFunc     func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeUserRepository) Create(ctx context.Context, input domain.CreateUserInput, passwordHash string) (*domain.User, error) {
	return f.CreateFunc(ctx, input, passwordHash)
}

func (f *fakeUserRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return f.GetByUUIDFunc(ctx, id)
}

func (f *fakeUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return f.GetByEmailFunc(ctx, email)
}

func (f *fakeUserRepository) List(ctx context.Context, filter domain.UserListFilter) ([]*domain.User, int64, error) {
	return f.ListFunc(ctx, filter)
}

func (f *fakeUserRepository) Update(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	return f.UpdateFunc(ctx, id, input)
}

func (f *fakeUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return f.DeleteFunc(ctx, id)
}

// fakeStorage is a hand-written test double for storage.Service that records
// every UploadFile/DeleteFile call so tests can assert on call counts and args.
type fakeStorage struct {
	UploadFileFunc func(ctx context.Context, filename string) (string, error)
	DeleteFileFunc func(ctx context.Context, fileURL string) error

	uploadCalls []string
	deleteCalls []string
}

func (f *fakeStorage) UploadFile(ctx context.Context, file io.Reader, originalFilename string) (string, error) {
	f.uploadCalls = append(f.uploadCalls, originalFilename)
	if f.UploadFileFunc != nil {
		return f.UploadFileFunc(ctx, originalFilename)
	}
	return "", nil
}

func (f *fakeStorage) DeleteFile(ctx context.Context, fileURL string) error {
	f.deleteCalls = append(f.deleteCalls, fileURL)
	if f.DeleteFileFunc != nil {
		return f.DeleteFileFunc(ctx, fileURL)
	}
	return nil
}

func strPtr(s string) *string { return &s }

func TestUserService_CreateUser(t *testing.T) {
	t.Run("hashes the password and persists it, not the plaintext", func(t *testing.T) {
		var gotHash string
		var gotInput domain.CreateUserInput
		repo := &fakeUserRepository{
			CreateFunc: func(_ context.Context, input domain.CreateUserInput, passwordHash string) (*domain.User, error) {
				gotInput = input
				gotHash = passwordHash
				return &domain.User{Email: input.Email}, nil
			},
		}
		svc := NewUserService(repo, &fakeStorage{})

		input := domain.CreateUserInput{FirstName: "Jane", LastName: "Doe", Email: "jane@example.com", Password: "s3cret-pw"}
		user, err := svc.CreateUser(context.Background(), input)
		if err != nil {
			t.Fatalf("CreateUser() error = %v, want nil", err)
		}
		if user.Email != input.Email {
			t.Errorf("repo received email %q, want %q", gotInput.Email, input.Email)
		}
		if gotHash == "" || gotHash == input.Password {
			t.Fatalf("repo received passwordHash %q, want a bcrypt hash distinct from the plaintext", gotHash)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(gotHash), []byte(input.Password)); err != nil {
			t.Errorf("stored hash does not match the original password: %v", err)
		}
	})

	t.Run("propagates a repository error without wrapping", func(t *testing.T) {
		wantErr := errors.New("unique violation")
		repo := &fakeUserRepository{
			CreateFunc: func(context.Context, domain.CreateUserInput, string) (*domain.User, error) {
				return nil, wantErr
			},
		}
		svc := NewUserService(repo, &fakeStorage{})

		_, err := svc.CreateUser(context.Background(), domain.CreateUserInput{Password: "whatever"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("CreateUser() error = %v, want %v", err, wantErr)
		}
	})
}

func TestUserService_UploadAvatar(t *testing.T) {
	userID := uuid.New()

	t.Run("uploads the new file, updates the record, then deletes the old avatar", func(t *testing.T) {
		var deletedBeforeUpdate bool
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: strPtr("old/avatar.png")}, nil
			},
			UpdateFunc: func(_ context.Context, _ uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
				if input.AvatarUrl == nil || *input.AvatarUrl != "new/avatar.png" {
					t.Errorf("Update() got AvatarUrl %v, want \"new/avatar.png\"", input.AvatarUrl)
				}
				return &domain.User{Uuid: userID, AvatarUrl: input.AvatarUrl}, nil
			},
		}
		storage := &fakeStorage{
			UploadFileFunc: func(context.Context, string) (string, error) { return "new/avatar.png", nil },
			DeleteFileFunc: func(_ context.Context, fileURL string) error {
				deletedBeforeUpdate = true
				if fileURL != "old/avatar.png" {
					t.Errorf("DeleteFile() got %q, want \"old/avatar.png\"", fileURL)
				}
				return nil
			},
		}
		svc := NewUserService(repo, storage)

		user, err := svc.UploadAvatar(context.Background(), userID, domain.AvatarFile{Filename: "me.png"})
		if err != nil {
			t.Fatalf("UploadAvatar() error = %v, want nil", err)
		}
		if !deletedBeforeUpdate {
			t.Error("old avatar was never deleted")
		}
		if *user.AvatarUrl != "new/avatar.png" {
			t.Errorf("returned user AvatarUrl = %q, want \"new/avatar.png\"", *user.AvatarUrl)
		}
	})

	t.Run("does not attempt to delete anything when the user had no avatar", func(t *testing.T) {
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: nil}, nil
			},
			UpdateFunc: func(_ context.Context, _ uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: input.AvatarUrl}, nil
			},
		}
		storage := &fakeStorage{
			UploadFileFunc: func(context.Context, string) (string, error) { return "new/avatar.png", nil },
		}
		svc := NewUserService(repo, storage)

		if _, err := svc.UploadAvatar(context.Background(), userID, domain.AvatarFile{Filename: "me.png"}); err != nil {
			t.Fatalf("UploadAvatar() error = %v, want nil", err)
		}
		if len(storage.deleteCalls) != 0 {
			t.Errorf("DeleteFile called %d times, want 0", len(storage.deleteCalls))
		}
	})

	t.Run("returns ErrInvalidAvatarFile and does not touch the record when upload fails", func(t *testing.T) {
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID}, nil
			},
			UpdateFunc: func(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
				t.Fatal("Update should not be called when upload fails")
				return nil, nil
			},
		}
		storage := &fakeStorage{
			UploadFileFunc: func(context.Context, string) (string, error) { return "", errors.New("disk full") },
		}
		svc := NewUserService(repo, storage)

		_, err := svc.UploadAvatar(context.Background(), userID, domain.AvatarFile{Filename: "me.png"})
		if !errors.Is(err, domain.ErrInvalidAvatarFile) {
			t.Fatalf("UploadAvatar() error = %v, want %v", err, domain.ErrInvalidAvatarFile)
		}
	})

	t.Run("cleans up the newly uploaded file when Update fails, leaving the old avatar untouched", func(t *testing.T) {
		wantErr := errors.New("db unavailable")
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: strPtr("old/avatar.png")}, nil
			},
			UpdateFunc: func(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
				return nil, wantErr
			},
		}
		storage := &fakeStorage{
			UploadFileFunc: func(context.Context, string) (string, error) { return "new/avatar.png", nil },
		}
		svc := NewUserService(repo, storage)

		_, err := svc.UploadAvatar(context.Background(), userID, domain.AvatarFile{Filename: "me.png"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("UploadAvatar() error = %v, want %v", err, wantErr)
		}
		if len(storage.deleteCalls) != 1 || storage.deleteCalls[0] != "new/avatar.png" {
			t.Errorf("DeleteFile calls = %v, want exactly [\"new/avatar.png\"] (the orphaned upload must be cleaned up, the old avatar left alone)", storage.deleteCalls)
		}
	})

	t.Run("propagates a GetByUUID error before touching storage", func(t *testing.T) {
		wantErr := errors.New("user not found")
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, wantErr },
		}
		storage := &fakeStorage{}
		svc := NewUserService(repo, storage)

		_, err := svc.UploadAvatar(context.Background(), userID, domain.AvatarFile{Filename: "me.png"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("UploadAvatar() error = %v, want %v", err, wantErr)
		}
		if len(storage.uploadCalls) != 0 {
			t.Errorf("UploadFile called %d times, want 0", len(storage.uploadCalls))
		}
	})
}

func TestUserService_DeleteAvatar(t *testing.T) {
	userID := uuid.New()

	t.Run("deletes the file and clears the avatar URL", func(t *testing.T) {
		var gotClearedTo *string
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: strPtr("avatar.png")}, nil
			},
			UpdateFunc: func(_ context.Context, _ uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
				gotClearedTo = input.AvatarUrl
				return &domain.User{Uuid: userID, AvatarUrl: input.AvatarUrl}, nil
			},
		}
		storage := &fakeStorage{}
		svc := NewUserService(repo, storage)

		if _, err := svc.DeleteAvatar(context.Background(), userID); err != nil {
			t.Fatalf("DeleteAvatar() error = %v, want nil", err)
		}
		if len(storage.deleteCalls) != 1 || storage.deleteCalls[0] != "avatar.png" {
			t.Errorf("DeleteFile calls = %v, want exactly [\"avatar.png\"]", storage.deleteCalls)
		}
		if gotClearedTo == nil || *gotClearedTo != "" {
			t.Errorf("Update() got AvatarUrl %v, want a pointer to an empty string", gotClearedTo)
		}
	})

	t.Run("is a no-op when the user has no avatar", func(t *testing.T) {
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: nil}, nil
			},
			UpdateFunc: func(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
				t.Fatal("Update should not be called when there is no avatar to clear")
				return nil, nil
			},
		}
		storage := &fakeStorage{}
		svc := NewUserService(repo, storage)

		if _, err := svc.DeleteAvatar(context.Background(), userID); err != nil {
			t.Fatalf("DeleteAvatar() error = %v, want nil", err)
		}
		if len(storage.deleteCalls) != 0 {
			t.Errorf("DeleteFile called %d times, want 0", len(storage.deleteCalls))
		}
	})

	t.Run("treats an already-empty avatar URL as no avatar", func(t *testing.T) {
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) {
				return &domain.User{Uuid: userID, AvatarUrl: strPtr("")}, nil
			},
		}
		storage := &fakeStorage{}
		svc := NewUserService(repo, storage)

		if _, err := svc.DeleteAvatar(context.Background(), userID); err != nil {
			t.Fatalf("DeleteAvatar() error = %v, want nil", err)
		}
		if len(storage.deleteCalls) != 0 {
			t.Errorf("DeleteFile called %d times, want 0", len(storage.deleteCalls))
		}
	})

	t.Run("propagates a GetByUUID error", func(t *testing.T) {
		wantErr := errors.New("user not found")
		repo := &fakeUserRepository{
			GetByUUIDFunc: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, wantErr },
		}
		svc := NewUserService(repo, &fakeStorage{})

		_, err := svc.DeleteAvatar(context.Background(), userID)
		if !errors.Is(err, wantErr) {
			t.Fatalf("DeleteAvatar() error = %v, want %v", err, wantErr)
		}
	})
}

func TestUserService_GetUser(t *testing.T) {
	userID := uuid.New()
	want := &domain.User{Uuid: userID, Email: "jane@example.com"}
	repo := &fakeUserRepository{
		GetByUUIDFunc: func(_ context.Context, id uuid.UUID) (*domain.User, error) {
			if id != userID {
				t.Errorf("GetByUUID called with %v, want %v", id, userID)
			}
			return want, nil
		},
	}
	svc := NewUserService(repo, &fakeStorage{})

	got, err := svc.GetUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUser() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("GetUser() = %v, want %v", got, want)
	}
}

func TestUserService_GetUsers(t *testing.T) {
	filter := domain.UserListFilter{Search: "jane", Limit: 10, Offset: 0, SortOrder: "asc"}
	wantUsers := []*domain.User{{Email: "jane@example.com"}}
	repo := &fakeUserRepository{
		ListFunc: func(_ context.Context, f domain.UserListFilter) ([]*domain.User, int64, error) {
			if f != filter {
				t.Errorf("List called with %+v, want %+v", f, filter)
			}
			return wantUsers, 1, nil
		},
	}
	svc := NewUserService(repo, &fakeStorage{})

	users, total, err := svc.GetUsers(context.Background(), filter)
	if err != nil {
		t.Fatalf("GetUsers() error = %v, want nil", err)
	}
	if total != 1 || len(users) != 1 {
		t.Errorf("GetUsers() = (%v, %d), want ([1 user], 1)", users, total)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	userID := uuid.New()
	input := domain.UpdateUserInput{FirstName: strPtr("New")}
	want := &domain.User{Uuid: userID, FirstName: "New"}
	repo := &fakeUserRepository{
		UpdateFunc: func(_ context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
			if id != userID || in.FirstName == nil || *in.FirstName != "New" {
				t.Errorf("Update called with (%v, %+v), want (%v, %+v)", id, in, userID, input)
			}
			return want, nil
		},
	}
	svc := NewUserService(repo, &fakeStorage{})

	got, err := svc.UpdateUser(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v, want nil", err)
	}
	if got != want {
		t.Errorf("UpdateUser() = %v, want %v", got, want)
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	t.Run("delegates to the repository", func(t *testing.T) {
		userID := uuid.New()
		var gotID uuid.UUID
		repo := &fakeUserRepository{
			DeleteFunc: func(_ context.Context, id uuid.UUID) error {
				gotID = id
				return nil
			},
		}
		svc := NewUserService(repo, &fakeStorage{})

		if err := svc.DeleteUser(context.Background(), userID); err != nil {
			t.Fatalf("DeleteUser() error = %v, want nil", err)
		}
		if gotID != userID {
			t.Errorf("Delete called with %v, want %v", gotID, userID)
		}
	})

	t.Run("propagates a repository error", func(t *testing.T) {
		wantErr := errors.New("not found")
		repo := &fakeUserRepository{
			DeleteFunc: func(context.Context, uuid.UUID) error { return wantErr },
		}
		svc := NewUserService(repo, &fakeStorage{})

		if err := svc.DeleteUser(context.Background(), uuid.New()); !errors.Is(err, wantErr) {
			t.Fatalf("DeleteUser() error = %v, want %v", err, wantErr)
		}
	})
}
