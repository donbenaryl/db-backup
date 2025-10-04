#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are installed
check_requirements() {
    print_status "Checking requirements..."
    
    if ! command -v terraform &> /dev/null; then
        print_error "Terraform is not installed. Please install Terraform first."
        exit 1
    fi
    
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go first."
        exit 1
    fi
    
    if ! command -v aws &> /dev/null; then
        print_error "AWS CLI is not installed. Please install AWS CLI first."
        exit 1
    fi
    
    print_success "All requirements are met"
}

# Build the Lambda function
build_lambda() {
    print_status "Building Lambda function..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    
    cd "$DEPLOY_DIR/.."
    
    # Build for Linux (Lambda runtime)
    GOOS=linux GOARCH=amd64 go build -o cmd/lambda/main cmd/lambda/main.go
    
    if [ $? -eq 0 ]; then
        print_success "Lambda function built successfully"
    else
        print_error "Failed to build Lambda function"
        exit 1
    fi
    
    # Return to deploy directory
    cd "$DEPLOY_DIR"
}

# Initialize Terraform
init_terraform() {
    print_status "Initializing Terraform..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    
    terraform init
    
    if [ $? -eq 0 ]; then
        print_success "Terraform initialized successfully"
    else
        print_error "Failed to initialize Terraform"
        exit 1
    fi
}

# Plan Terraform deployment
plan_terraform() {
    print_status "Planning Terraform deployment..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    
    terraform plan -out=tfplan
    
    if [ $? -eq 0 ]; then
        print_success "Terraform plan created successfully"
    else
        print_error "Failed to create Terraform plan"
        exit 1
    fi
}

# Apply Terraform deployment
apply_terraform() {
    print_status "Applying Terraform deployment..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    
    terraform apply tfplan
    
    if [ $? -eq 0 ]; then
        print_success "Terraform deployment completed successfully"
    else
        print_error "Failed to apply Terraform deployment"
        exit 1
    fi
}

# Clean up
cleanup() {
    print_status "Cleaning up..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    
    rm -f tfplan
    rm -f "$DEPLOY_DIR/../cmd/lambda/main"
    
    print_success "Cleanup completed"
}

# Main deployment function
deploy() {
    print_status "Starting PostgreSQL Backup Service (backup-only) deployment to AWS Lambda..."
    
    check_requirements
    build_lambda
    init_terraform
    plan_terraform
    
    # Ask for confirmation
    echo
    print_warning "This will deploy the PostgreSQL Backup Service (backup-only) to AWS Lambda."
    print_warning "The S3 bucket will be created and configured to never be deleted."
    echo
    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        apply_terraform
        cleanup
        
        echo
        print_success "Deployment completed successfully!"
        echo
        print_status "Useful commands:"
        echo "  terraform output                    # View all outputs"
        echo "  terraform output s3_bucket_name    # Get S3 bucket name"
        echo "  terraform output lambda_function_name # Get Lambda function name"
        echo "  aws logs tail /aws/lambda/$(terraform output -raw lambda_function_name) --follow # Follow Lambda logs"
        echo
        print_status "The backup service will run every 3 hours automatically."
        print_status "Backup files will be stored in the S3 bucket with a 2-day retention policy."
    else
        print_warning "Deployment cancelled by user"
        cleanup
        exit 0
    fi
}

# Destroy function
destroy() {
    print_status "Destroying Terraform resources..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    
    print_warning "This will destroy most resources, but the S3 bucket will be preserved."
    print_warning "The S3 bucket is configured to never be deleted for data safety."
    echo
    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        terraform destroy -auto-approve
        
        if [ $? -eq 0 ]; then
            print_success "Resources destroyed successfully"
            print_warning "S3 bucket has been preserved as configured"
        else
            print_error "Failed to destroy resources"
            exit 1
        fi
    else
        print_warning "Destroy cancelled by user"
    fi
}

# Show help
show_help() {
    echo "PostgreSQL Backup Service - AWS Lambda Deployment Script"
    echo
    echo "Usage: $0 [COMMAND]"
    echo
    echo "Commands:"
    echo "  deploy    Deploy the service to AWS Lambda (default)"
    echo "  destroy   Destroy the deployed resources (preserves S3 bucket)"
    echo "  plan      Show Terraform plan without applying"
    echo "  output    Show Terraform outputs"
    echo "  logs      Show Lambda function logs"
    echo "  help      Show this help message"
    echo
    echo "Examples:"
    echo "  $0 deploy     # Deploy the service"
    echo "  $0 destroy    # Destroy resources (keeps S3 bucket)"
    echo "  $0 plan       # Show what will be deployed"
    echo "  $0 output     # Show deployment outputs"
    echo "  $0 logs       # Show Lambda logs"
}

# Show Terraform plan
show_plan() {
    print_status "Showing Terraform plan..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    terraform plan
}

# Show Terraform outputs
show_outputs() {
    print_status "Showing Terraform outputs..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    terraform output
}

# Show Lambda logs
show_logs() {
    print_status "Showing Lambda function logs..."
    
    # Get absolute path to deploy directory
    DEPLOY_DIR="$(cd "$(dirname "$0")" && pwd)"
    cd "$DEPLOY_DIR"
    
    function_name=$(terraform output -raw lambda_function_name 2>/dev/null)
    if [ $? -eq 0 ] && [ -n "$function_name" ]; then
        aws logs tail "/aws/lambda/$function_name" --follow
    else
        print_error "Could not get Lambda function name. Make sure the service is deployed."
        exit 1
    fi
}

# Main script logic
case "${1:-deploy}" in
    deploy)
        deploy
        ;;
    destroy)
        destroy
        ;;
    plan)
        check_requirements
        build_lambda
        init_terraform
        show_plan
        cleanup
        ;;
    output)
        show_outputs
        ;;
    logs)
        show_logs
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        show_help
        exit 1
        ;;
esac