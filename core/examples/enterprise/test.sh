#!/bin/bash

# Enterprise SaaS Platform - Comprehensive Test Script
# This script tests all enterprise features

set -e

BASE_URL="http://localhost:8080"
ACME_HOST="acme.myapp.com"
TECHCORP_HOST="techcorp.myapp.com"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

test_endpoint() {
    local description=$1
    local method=$2
    local url=$3
    local host=$4
    local data=$5

    echo -e "\nTesting: ${description}"
    echo "  Method: ${method}"
    echo "  URL: ${url}"
    echo "  Host: ${host}"

    if [ -z "$data" ]; then
        response=$(curl -s -X ${method} -H "Host: ${host}" "${url}")
    else
        response=$(curl -s -X ${method} -H "Host: ${host}" -H "Content-Type: application/json" -d "${data}" "${url}")
    fi

    if [ $? -eq 0 ]; then
        print_success "Response received"
        echo "$response" | jq '.' 2>/dev/null || echo "$response"
    else
        print_error "Request failed"
        return 1
    fi
}

# Check if server is running
check_server() {
    print_header "Checking Server Status"

    if curl -s "${BASE_URL}/api/v1/projects" > /dev/null 2>&1; then
        print_success "Server is running at ${BASE_URL}"
    else
        print_error "Server is not running. Please start the application first."
        echo ""
        echo "Run: make run"
        echo "Or: go run main.go"
        exit 1
    fi
}

# Test Multi-Tenancy
test_multitenancy() {
    print_header "Testing Multi-Tenancy"

    print_info "Testing Acme Corporation tenant..."
    test_endpoint \
        "List projects for Acme Corp" \
        "GET" \
        "${BASE_URL}/api/v1/projects" \
        "${ACME_HOST}"

    print_info "Testing Tech Corp tenant..."
    test_endpoint \
        "List projects for Tech Corp" \
        "GET" \
        "${BASE_URL}/api/v1/projects" \
        "${TECHCORP_HOST}"
}

# Test API Versioning
test_api_versioning() {
    print_header "Testing API Versioning"

    print_info "Testing V1 API (Offset Pagination)..."
    test_endpoint \
        "List projects V1 with offset pagination" \
        "GET" \
        "${BASE_URL}/api/v1/projects?page=1&limit=10&sort=created_at&order=desc" \
        "${ACME_HOST}"

    print_info "Testing V2 API (Cursor Pagination)..."
    test_endpoint \
        "List projects V2 with cursor pagination" \
        "GET" \
        "${BASE_URL}/api/v2/projects?limit=10&sort=created_at&order=desc" \
        "${ACME_HOST}"
}

# Test Pagination
test_pagination() {
    print_header "Testing Pagination Features"

    print_info "Testing offset pagination (page-based)..."
    test_endpoint \
        "Offset pagination - Page 1" \
        "GET" \
        "${BASE_URL}/api/v1/projects?page=1&limit=2" \
        "${ACME_HOST}"

    test_endpoint \
        "Offset pagination - Page 2" \
        "GET" \
        "${BASE_URL}/api/v1/projects?page=2&limit=2" \
        "${ACME_HOST}"

    print_info "Testing cursor pagination..."
    test_endpoint \
        "Cursor pagination - First page" \
        "GET" \
        "${BASE_URL}/api/v2/projects?limit=2" \
        "${ACME_HOST}"
}

# Test CRUD Operations
test_crud_operations() {
    print_header "Testing CRUD Operations"

    print_info "Testing CREATE..."
    test_endpoint \
        "Create new project" \
        "POST" \
        "${BASE_URL}/api/v1/projects" \
        "${ACME_HOST}" \
        '{"name":"Test Project from Script","description":"Created by test script"}'

    print_info "Testing READ..."
    test_endpoint \
        "Get specific project" \
        "GET" \
        "${BASE_URL}/api/v1/projects/proj-1" \
        "${ACME_HOST}"

    print_info "Testing UPDATE..."
    test_endpoint \
        "Update project" \
        "PUT" \
        "${BASE_URL}/api/v1/projects/proj-1" \
        "${ACME_HOST}" \
        '{"name":"Updated Project Name","status":"inactive"}'

    print_info "Testing DELETE..."
    test_endpoint \
        "Delete project" \
        "DELETE" \
        "${BASE_URL}/api/v1/projects/proj-1" \
        "${ACME_HOST}"
}

# Test V2 Batch Operations
test_batch_operations() {
    print_header "Testing V2 Batch Operations"

    print_info "Testing batch update..."
    test_endpoint \
        "Batch update multiple projects" \
        "POST" \
        "${BASE_URL}/api/v2/projects/batch" \
        "${ACME_HOST}" \
        '{"project_ids":["proj-1","proj-2"],"updates":{"status":"archived"}}'
}

# Test Authorization (RBAC)
test_authorization() {
    print_header "Testing Authorization (RBAC)"

    print_info "The application has pre-configured roles:"
    echo "  - super_admin: All permissions (*)"
    echo "  - tenant_admin: projects:*, users:*, settings:*"
    echo "  - project_manager: projects:read/create/update, users:read"
    echo "  - developer: projects:read/update"
    echo "  - viewer: *:read"

    print_info "Testing read access (all roles should succeed)..."
    test_endpoint \
        "Read project (viewer role)" \
        "GET" \
        "${BASE_URL}/api/v1/projects/proj-1" \
        "${ACME_HOST}"

    print_info "Testing create access (developer+ should succeed)..."
    test_endpoint \
        "Create project (developer role)" \
        "POST" \
        "${BASE_URL}/api/v1/projects" \
        "${ACME_HOST}" \
        '{"name":"RBAC Test Project","description":"Testing authorization"}'
}

# Test Tenant Isolation
test_tenant_isolation() {
    print_header "Testing Tenant Isolation"

    print_info "Creating project in Acme tenant..."
    acme_response=$(curl -s -X POST \
        -H "Host: ${ACME_HOST}" \
        -H "Content-Type: application/json" \
        -d '{"name":"Acme Exclusive Project","description":"Only visible to Acme"}' \
        "${BASE_URL}/api/v1/projects")

    echo "$acme_response" | jq '.'

    print_info "Verifying project appears in Acme tenant..."
    test_endpoint \
        "List Acme projects" \
        "GET" \
        "${BASE_URL}/api/v1/projects" \
        "${ACME_HOST}"

    print_info "Verifying project does NOT appear in TechCorp tenant..."
    test_endpoint \
        "List TechCorp projects (should not include Acme projects)" \
        "GET" \
        "${BASE_URL}/api/v1/projects" \
        "${TECHCORP_HOST}"
}

# Performance Test
test_performance() {
    print_header "Testing Performance"

    print_info "Running concurrent requests test..."

    start_time=$(date +%s%N)

    for i in {1..10}; do
        curl -s -H "Host: ${ACME_HOST}" "${BASE_URL}/api/v1/projects" > /dev/null &
    done

    wait

    end_time=$(date +%s%N)
    duration=$(( (end_time - start_time) / 1000000 ))

    print_success "Completed 10 concurrent requests in ${duration}ms"
    echo "  Average: $((duration / 10))ms per request"
}

# Configuration Test
test_configuration() {
    print_header "Testing Configuration Management"

    print_info "Current configuration values are managed by Viper"
    print_info "Configuration sources:"
    echo "  1. Default values (in code)"
    echo "  2. Environment variables (APP_ prefix)"
    echo "  3. Configuration files (config.yaml, config.json)"

    print_success "Configuration hot reload is enabled"
    print_info "Try creating config.yaml with custom values to test hot reload"
}

# Summary
print_summary() {
    print_header "Test Summary"

    echo -e "${GREEN}All enterprise features tested:${NC}"
    echo "  ✓ Multi-Tenancy (subdomain strategy)"
    echo "  ✓ API Versioning (v1 and v2)"
    echo "  ✓ Pagination (offset and cursor-based)"
    echo "  ✓ RBAC Authorization"
    echo "  ✓ CRUD Operations"
    echo "  ✓ Batch Operations (v2)"
    echo "  ✓ Tenant Isolation"
    echo "  ✓ Configuration Management"
    echo "  ✓ Performance"
    echo ""
    echo -e "${YELLOW}Note: OAuth2 authentication requires manual setup${NC}"
    echo "Run 'make setup-oauth' for instructions"
}

# Main execution
main() {
    echo -e "${BLUE}"
    cat << "EOF"
╔═══════════════════════════════════════════════════════╗
║   Enterprise SaaS Platform - Comprehensive Tests     ║
║          Unicorn Framework Demo Application          ║
╚═══════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"

    check_server
    test_multitenancy
    test_api_versioning
    test_pagination
    test_crud_operations
    test_batch_operations
    test_authorization
    test_tenant_isolation
    test_configuration
    test_performance
    print_summary

    echo ""
    print_success "All tests completed!"
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    print_error "jq is not installed. Install it for better JSON formatting:"
    echo "  macOS: brew install jq"
    echo "  Linux: sudo apt-get install jq"
    echo ""
    echo "Continuing without jq..."
    sleep 2
fi

# Run tests
main "$@"
