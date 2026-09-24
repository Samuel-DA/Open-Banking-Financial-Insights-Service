data "aws_availability_zones" "available" { state = "available" }
resource "aws_vpc" "main" { cidr_block = var.vpc_cidr; enable_dns_hostnames = true; tags = { Name = "insights-${var.environment}" } }
resource "aws_subnet" "private" { count = 2; vpc_id = aws_vpc.main.id; cidr_block = cidrsubnet(var.vpc_cidr, 4, count.index); availability_zone = data.aws_availability_zones.available.names[count.index]; tags = { "kubernetes.io/role/internal-elb" = "1" } }
resource "aws_ecr_repository" "api" { name = "insights-api"; image_scanning_configuration { scan_on_push = true }; encryption_configuration { encryption_type = "AES256" } }
resource "aws_ecr_repository" "worker" { name = "insights-worker"; image_scanning_configuration { scan_on_push = true }; encryption_configuration { encryption_type = "AES256" } }
resource "aws_security_group" "data" { name = "insights-data"; vpc_id = aws_vpc.main.id; ingress { from_port = 5432; to_port = 5432; protocol = "tcp"; cidr_blocks = [var.vpc_cidr] }; ingress { from_port = 9092; to_port = 9092; protocol = "tcp"; cidr_blocks = [var.vpc_cidr] } }
resource "aws_db_subnet_group" "main" { name = "insights-${var.environment}"; subnet_ids = aws_subnet.private[*].id }
resource "aws_db_instance" "postgres" { identifier = "insights-${var.environment}"; engine = "postgres"; engine_version = "16"; instance_class = "db.t4g.micro"; allocated_storage = 20; db_name = "insights"; username = "insights_admin"; manage_master_user_password = true; db_subnet_group_name = aws_db_subnet_group.main.name; vpc_security_group_ids = [aws_security_group.data.id]; storage_encrypted = true; skip_final_snapshot = true; deletion_protection = false; publicly_accessible = false }
resource "aws_msk_serverless_cluster" "kafka" { cluster_name = "insights-${var.environment}"; vpc_config { subnet_ids = aws_subnet.private[*].id; security_group_ids = [aws_security_group.data.id] }; client_authentication { sasl { iam { enabled = true } } } }
resource "aws_iam_role" "eks" { name = "insights-eks-${var.environment}"; assume_role_policy = jsonencode({ Version="2012-10-17", Statement=[{Action="sts:AssumeRole",Effect="Allow",Principal={Service="eks.amazonaws.com"}}] }) }
resource "aws_iam_role_policy_attachment" "eks" { role = aws_iam_role.eks.name; policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy" }
resource "aws_eks_cluster" "main" { name = "insights-${var.environment}"; role_arn = aws_iam_role.eks.arn; vpc_config { subnet_ids = aws_subnet.private[*].id }; depends_on = [aws_iam_role_policy_attachment.eks] }
output "ecr_api_url" { value = aws_ecr_repository.api.repository_url }
output "database_endpoint" { value = aws_db_instance.postgres.endpoint; sensitive = true }
output "kafka_bootstrap_brokers" { value = aws_msk_serverless_cluster.kafka.bootstrap_brokers_sasl_iam }
