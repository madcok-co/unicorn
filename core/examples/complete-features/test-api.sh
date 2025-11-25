#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

echo -e "${CYAN}🦄 Unicorn API Testing Script${NC}\n"

# Function to print test result
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ $2${NC}"
    else
        echo -e "${RED}✗ $2${NC}"
    fi
}

# Test 1: Health Check
echo -e "${YELLOW}Test 1: Health Check${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json ${BASE_URL}/health)
if [ "$response" = "200" ]; then
    print_result 0 "Health check passed"
    cat /tmp/response.json | jq .
else
    print_result 1 "Health check failed (HTTP $response)"
fi
echo ""

# Test 2: Register User
echo -e "${YELLOW}Test 2: Register User${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "TestPass123!"
  }')

if [ "$response" = "200" ]; then
    print_result 0 "User registration successful"
    cat /tmp/response.json | jq .
else
    print_result 1 "User registration failed (HTTP $response)"
    cat /tmp/response.json
fi
echo ""

# Test 3: Login
echo -e "${YELLOW}Test 3: Login${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "TestPass123!"
  }')

if [ "$response" = "200" ]; then
    print_result 0 "Login successful"
    TOKEN=$(cat /tmp/response.json | jq -r '.token')
    echo -e "${CYAN}Token: ${TOKEN:0:50}...${NC}"
    cat /tmp/response.json | jq .
else
    print_result 1 "Login failed (HTTP $response)"
    cat /tmp/response.json
    exit 1
fi
echo ""

# Test 4: Verify Token
echo -e "${YELLOW}Test 4: Verify Token${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/auth/verify \
  -H "Authorization: Bearer ${TOKEN}")

if [ "$response" = "200" ]; then
    print_result 0 "Token verification successful"
    cat /tmp/response.json | jq .
else
    print_result 1 "Token verification failed (HTTP $response)"
fi
echo ""

# Test 5: Get Profile
echo -e "${YELLOW}Test 5: Get Profile${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json ${BASE_URL}/auth/profile \
  -H "Authorization: Bearer ${TOKEN}")

if [ "$response" = "200" ]; then
    print_result 0 "Get profile successful"
    cat /tmp/response.json | jq .
else
    print_result 1 "Get profile failed (HTTP $response)"
fi
echo ""

# Test 6: Create Product
echo -e "${YELLOW}Test 6: Create Product${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/products \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "name": "Test Laptop",
    "description": "High-performance laptop for testing",
    "price": 1299.99,
    "stock": 10
  }')

if [ "$response" = "200" ]; then
    print_result 0 "Create product successful"
    PRODUCT_ID=$(cat /tmp/response.json | jq -r '.id')
    echo -e "${CYAN}Product ID: ${PRODUCT_ID}${NC}"
    cat /tmp/response.json | jq .
else
    print_result 1 "Create product failed (HTTP $response)"
fi
echo ""

# Test 7: List Products
echo -e "${YELLOW}Test 7: List Products${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json ${BASE_URL}/products)

if [ "$response" = "200" ]; then
    print_result 0 "List products successful"
    cat /tmp/response.json | jq .
else
    print_result 1 "List products failed (HTTP $response)"
fi
echo ""

# Test 8: Get Product by ID
if [ ! -z "$PRODUCT_ID" ]; then
    echo -e "${YELLOW}Test 8: Get Product by ID${NC}"
    response=$(curl -s -w "%{http_code}" -o /tmp/response.json ${BASE_URL}/products/${PRODUCT_ID})

    if [ "$response" = "200" ]; then
        print_result 0 "Get product by ID successful"
        cat /tmp/response.json | jq .
    else
        print_result 1 "Get product by ID failed (HTTP $response)"
    fi
    echo ""
fi

# Test 9: Invalid Login
echo -e "${YELLOW}Test 9: Invalid Login (should fail)${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "WrongPassword"
  }')

if [ "$response" != "200" ]; then
    print_result 0 "Invalid login correctly rejected"
else
    print_result 1 "Invalid login should have been rejected"
fi
echo ""

# Test 10: Invalid Token
echo -e "${YELLOW}Test 10: Invalid Token (should fail)${NC}"
response=$(curl -s -w "%{http_code}" -o /tmp/response.json -X POST ${BASE_URL}/auth/verify \
  -H "Authorization: Bearer invalid-token-here")

if [ "$response" != "200" ]; then
    print_result 0 "Invalid token correctly rejected"
else
    print_result 1 "Invalid token should have been rejected"
fi
echo ""

echo -e "${GREEN}✅ All tests completed!${NC}"
echo -e "${CYAN}Check the results above for any failures${NC}"

# Cleanup
rm -f /tmp/response.json
