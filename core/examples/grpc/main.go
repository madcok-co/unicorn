package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	grpcAdapter "github.com/madcok-co/unicorn/contrib/grpc"
	"github.com/madcok-co/unicorn/core/examples/grpc/pb"
	"github.com/madcok-co/unicorn/core/pkg/app"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// In-memory storage for demo
var users = make(map[string]*pb.User)

// userServiceServer implements the UserService gRPC service
type userServiceServer struct {
	pb.UnimplementedUserServiceServer
}

// CreateUser creates a new user
func (s *userServiceServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	// Validate input
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	// Create user
	user := &pb.User{
		Id:        uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Role:      req.Role,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Store user
	users[user.Id] = user

	log.Printf("✅ Created user: %s (%s)", user.Name, user.Id)

	return &pb.CreateUserResponse{
		User:    user,
		Message: "User created successfully",
	}, nil
}

// GetUser retrieves a user by ID
func (s *userServiceServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, exists := users[req.Id]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "user with ID %s not found", req.Id)
	}

	log.Printf("📖 Retrieved user: %s (%s)", user.Name, user.Id)

	return &pb.GetUserResponse{
		User: user,
	}, nil
}

// ListUsers lists all users with pagination
func (s *userServiceServer) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	// Default pagination
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	// Collect all users (filter by role if specified)
	var allUsers []*pb.User
	for _, user := range users {
		if req.Role == "" || user.Role == req.Role {
			allUsers = append(allUsers, user)
		}
	}

	// Calculate pagination
	total := int32(len(allUsers))
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		start = total
	}
	if end > total {
		end = total
	}

	// Get paginated results
	var paginatedUsers []*pb.User
	if start < total {
		paginatedUsers = allUsers[start:end]
	}

	log.Printf("📋 Listed %d users (page %d, size %d)", len(paginatedUsers), page, pageSize)

	return &pb.ListUsersResponse{
		Users:    paginatedUsers,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// UpdateUser updates an existing user
func (s *userServiceServer) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	user, exists := users[req.Id]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "user with ID %s not found", req.Id)
	}

	// Update fields
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	user.UpdatedAt = time.Now().Unix()

	users[user.Id] = user

	log.Printf("✏️  Updated user: %s (%s)", user.Name, user.Id)

	return &pb.UpdateUserResponse{
		User:    user,
		Message: "User updated successfully",
	}, nil
}

// DeleteUser deletes a user by ID
func (s *userServiceServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	_, exists := users[req.Id]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "user with ID %s not found", req.Id)
	}

	delete(users, req.Id)

	log.Printf("🗑️  Deleted user: %s", req.Id)

	return &pb.DeleteUserResponse{
		Message: "User deleted successfully",
	}, nil
}

// StreamUsers streams users in real-time
func (s *userServiceServer) StreamUsers(req *pb.StreamUsersRequest, stream pb.UserService_StreamUsersServer) error {
	log.Printf("📡 Starting user stream (role filter: %s)", req.Role)

	// Stream existing users
	for _, user := range users {
		if req.Role == "" || user.Role == req.Role {
			if err := stream.Send(user); err != nil {
				return err
			}
			time.Sleep(100 * time.Millisecond) // Simulate streaming delay
		}
	}

	log.Printf("✅ Stream completed")
	return nil
}

func main() {
	// Create application
	application := app.New(&app.Config{
		Name:    "grpc-example",
		Version: "1.0.0",
	})

	// Create gRPC adapter
	grpcConfig := &grpcAdapter.Config{
		Host:             "0.0.0.0",
		Port:             9090,
		EnableReflection: true,
	}

	adapter := grpcAdapter.New(application.Registry(), grpcConfig)

	// Add recovery interceptor for panic handling
	adapter.UseUnaryInterceptor(grpcAdapter.RecoveryInterceptor())

	// Add custom logging interceptor
	adapter.UseUnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			log.Printf("❌ RPC %s failed in %v: %v", info.FullMethod, duration, err)
		} else {
			log.Printf("✅ RPC %s completed in %v", info.FullMethod, duration)
		}

		return resp, err
	})

	// Register user service
	userService := &userServiceServer{}
	adapter.RegisterService(&pb.UserService_ServiceDesc, userService)

	// Seed some initial data
	seedData()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Starting gRPC server on %s", adapter.Address())
		log.Printf("📝 Reflection enabled - use grpcurl for testing")
		log.Printf("\nExample commands:")
		log.Printf("  List services:")
		log.Printf("    grpcurl -plaintext localhost:9090 list")
		log.Printf("  Create user:")
		log.Printf("    grpcurl -plaintext -d '{\"name\":\"John Doe\",\"email\":\"john@example.com\",\"role\":\"admin\"}' localhost:9090 user.UserService/CreateUser")
		log.Printf("  List users:")
		log.Printf("    grpcurl -plaintext -d '{\"page\":1,\"page_size\":10}' localhost:9090 user.UserService/ListUsers")
		log.Printf("")

		if err := adapter.Start(ctx); err != nil {
			log.Printf("❌ Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("\n🛑 Shutting down gracefully...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Shutdown server
	if err := adapter.Shutdown(shutdownCtx); err != nil {
		log.Printf("❌ Shutdown error: %v", err)
	}

	log.Println("👋 Server stopped")
}

// seedData creates some initial users for testing
func seedData() {
	// Create admin user
	adminID := uuid.New().String()
	users[adminID] = &pb.User{
		Id:        adminID,
		Name:      "Admin User",
		Email:     "admin@example.com",
		Role:      "admin",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Create regular users
	for i := 1; i <= 5; i++ {
		userID := uuid.New().String()
		users[userID] = &pb.User{
			Id:        userID,
			Name:      fmt.Sprintf("User %d", i),
			Email:     fmt.Sprintf("user%d@example.com", i),
			Role:      "user",
			CreatedAt: time.Now().Unix(),
			UpdatedAt: time.Now().Unix(),
		}
	}

	log.Printf("✅ Seeded %d users", len(users))
}
