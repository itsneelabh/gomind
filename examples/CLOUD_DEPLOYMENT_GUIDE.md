# Cloud Provider Deployment Guide

This guide explains how to configure GoMind Framework deployments for different cloud providers by enabling and configuring Ingress resources and storage options.

## Overview

The GoMind Framework deployment files are configured to be cloud-agnostic by default. Ingress resources are commented out to ensure compatibility across all cloud providers. This guide shows you how to enable and configure these resources for your specific environment.

## Quick Start by Provider

### Amazon EKS (Elastic Kubernetes Service)

#### Prerequisites
- AWS Load Balancer Controller installed
- ACM certificates (for HTTPS/TLS)
- Appropriate IAM permissions

#### 1. Enable Ingress Resources
Uncomment the Ingress sections in your deployment files and update them:

```yaml
# For ALB (Application Load Balancer)
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-ingress
  namespace: gomind-examples
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:region:account:certificate/cert-id
    alb.ingress.kubernetes.io/ssl-policy: ELBSecurityPolicy-TLS-1-2-2017-01
    alb.ingress.kubernetes.io/ssl-redirect: '443'
spec:
  rules:
  - host: gomind.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: your-service-name
            port:
              number: 80
```

#### 2. Configure Storage Classes
Update PVC storage classes in `k8-deployment/` files:

```yaml
spec:
  storageClassName: gp3-csi  # Recommended
  # Alternative options:
  # storageClassName: gp2     # Legacy, cheaper
  # storageClassName: io1     # High IOPS
```

#### 3. Deploy
```bash
# Install AWS Load Balancer Controller
eksctl create iamserviceaccount \
  --cluster=your-cluster \
  --namespace=kube-system \
  --name=aws-load-balancer-controller \
  --role-name=AmazonEKSLoadBalancerControllerRole \
  --attach-policy-arn=arn:aws:iam::aws:policy/ElasticLoadBalancingFullAccess

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  --set clusterName=your-cluster \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller \
  -n kube-system

# Deploy GoMind
kubectl apply -f k8-deployment/
```

### Google GKE (Google Kubernetes Engine)

#### Prerequisites
- GCE Ingress Controller (default)
- Google-managed SSL certificates (optional)

#### 1. Enable Ingress Resources
Uncomment and configure Ingress resources:

```yaml
# For GCE Ingress
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-ingress
  namespace: gomind-examples
  annotations:
    kubernetes.io/ingress.class: "gce"
    ingress.gcp.kubernetes.io/managed-certificates: "gomind-ssl-cert"
    kubernetes.io/ingress.global-static-ip-name: "gomind-ip"
spec:
  rules:
  - host: gomind.yourdomain.com
    http:
      paths:
      - path: /*
        pathType: ImplementationSpecific
        backend:
          service:
            name: your-service-name
            port:
              number: 80
```

#### 2. Configure Storage Classes
```yaml
spec:
  storageClassName: premium-rwo  # SSD, recommended
  # Alternative options:
  # storageClassName: standard-rwo    # Standard disk
  # storageClassName: premium-rwx     # Multi-zone SSD
```

#### 3. Set up SSL Certificate (optional)
```yaml
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: gomind-ssl-cert
  namespace: gomind-examples
spec:
  domains:
    - gomind.yourdomain.com
```

#### 4. Deploy
```bash
# Reserve static IP
gcloud compute addresses create gomind-ip --global

# Deploy GoMind
kubectl apply -f k8-deployment/
```

### Microsoft AKS (Azure Kubernetes Service)

#### Prerequisites
- Application Gateway Ingress Controller or NGINX Ingress Controller
- Azure DNS zone (for custom domains)

#### 1. Enable Ingress Resources

##### Option A: Application Gateway Ingress Controller (AGIC)
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-ingress
  namespace: gomind-examples
  annotations:
    kubernetes.io/ingress.class: azure/application-gateway
    appgw.ingress.kubernetes.io/ssl-redirect: "true"
    appgw.ingress.kubernetes.io/request-timeout: "30"
spec:
  tls:
  - hosts:
    - gomind.yourdomain.com
    secretName: gomind-tls-secret
  rules:
  - host: gomind.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: your-service-name
            port:
              number: 80
```

##### Option B: NGINX Ingress Controller
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-ingress
  namespace: gomind-examples
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - gomind.yourdomain.com
    secretName: gomind-tls-secret
  rules:
  - host: gomind.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: your-service-name
            port:
              number: 80
```

#### 2. Configure Storage Classes
```yaml
spec:
  storageClassName: managed-csi  # Recommended
  # Alternative options:
  # storageClassName: managed-premium  # Premium SSD
  # storageClassName: azurefile        # Azure Files (ReadWriteMany)
```

#### 3. Deploy
```bash
# For NGINX Ingress Controller
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --create-namespace \
  --namespace ingress-nginx

# Deploy GoMind
kubectl apply -f k8-deployment/
```

### Kind (Local Development)

#### 1. Enable Ingress Resources
Uncomment the NGINX Ingress sections (already configured for Kind):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gomind-ingress
  namespace: gomind-examples
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: gomind.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: your-service-name
            port:
              number: 80
```

#### 2. Install NGINX Ingress Controller
```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

#### 3. Update /etc/hosts
```bash
# Add to /etc/hosts
127.0.0.1 gomind.local
127.0.0.1 grafana.local
127.0.0.1 prometheus.local
```

#### 4. Use setup-kind-demo.sh (Recommended)
The provided script handles everything automatically:
```bash
./setup-kind-demo.sh setup
```

## Configuration Examples by Service

### Grafana Ingress

#### EKS
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grafana-ingress
  namespace: gomind-examples
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
spec:
  rules:
  - host: grafana.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: grafana
            port:
              number: 80
```

#### GKE
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grafana-ingress
  namespace: gomind-examples
  annotations:
    kubernetes.io/ingress.class: "gce"
spec:
  rules:
  - host: grafana.yourdomain.com
    http:
      paths:
      - path: /*
        pathType: ImplementationSpecific
        backend:
          service:
            name: grafana
            port:
              number: 80
```

#### AKS
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grafana-ingress
  namespace: gomind-examples
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: grafana.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: grafana
            port:
              number: 80
```

## Storage Classes Summary

| Provider | Default | Premium | Multi-Zone | Use Case |
|----------|---------|---------|------------|----------|
| **EKS** | `gp3-csi` | `io1` | `efs-sc` | General purpose |
| **GKE** | `standard-rwo` | `premium-rwo` | `premium-rwx` | Performance |
| **AKS** | `managed-csi` | `managed-premium` | `azurefile` | Cost-effective |
| **Kind** | `standard` | N/A | N/A | Development |

## Security Considerations

### TLS/SSL Configuration

#### Automatic Certificate Management
```yaml
# For cert-manager (works with all providers)
annotations:
  cert-manager.io/cluster-issuer: letsencrypt-prod
```

#### Cloud-Specific Certificates
- **EKS**: Use ACM certificate ARNs
- **GKE**: Use Google-managed certificates
- **AKS**: Use Key Vault certificates or cert-manager

### Network Policies
All deployments include network policies. Enable them if your CNI supports them:
```bash
kubectl label namespace gomind-examples name=gomind-examples
```

## Troubleshooting

### Common Issues

#### 1. Ingress Not Working
```bash
# Check ingress controller
kubectl get pods -n ingress-nginx  # or kube-system

# Check ingress resource
kubectl describe ingress -n gomind-examples

# Check service endpoints
kubectl get endpoints -n gomind-examples
```

#### 2. Storage Issues
```bash
# List available storage classes
kubectl get storageclass

# Check PVC status
kubectl get pvc -n gomind-examples

# Describe problematic PVC
kubectl describe pvc <pvc-name> -n gomind-examples
```

#### 3. Certificate Issues
```bash
# Check certificate status (cert-manager)
kubectl get certificate -n gomind-examples

# Check certificate details
kubectl describe certificate <cert-name> -n gomind-examples
```

### Getting Help

#### Debug Commands
```bash
# Pod status
kubectl get pods -n gomind-examples -o wide

# Service status
kubectl get svc -n gomind-examples

# Ingress status
kubectl get ingress -n gomind-examples

# Events
kubectl get events -n gomind-examples --sort-by='.lastTimestamp'

# Logs
kubectl logs -n gomind-examples -l app.kubernetes.io/name=<component>
```

#### Port Forward (Alternative Access)
If Ingress is not working, use port forwarding:
```bash
# Grafana
kubectl port-forward -n gomind-examples svc/grafana 3000:80

# Prometheus
kubectl port-forward -n gomind-examples svc/prometheus 9090:9090

# Any GoMind service
kubectl port-forward -n gomind-examples svc/<service-name> 8080:80
```

## Next Steps

1. **Choose your cloud provider** from the options above
2. **Uncomment and modify** the Ingress resources in your deployment files
3. **Update storage classes** in PVC specifications
4. **Install required controllers** (Load Balancer, Ingress, etc.)
5. **Deploy** using `kubectl apply -f k8-deployment/`
6. **Configure DNS** to point to your load balancer/ingress
7. **Set up monitoring** using the provided Grafana dashboards

For local development and testing, we recommend using the Kind setup with the provided `setup-kind-demo.sh` script, which handles all the complexity automatically.