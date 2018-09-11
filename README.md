# Kubernetes SSL Approver

# Diagram

# Reason
- deploy kubernetes via kubeadm when using ssl mode, the ssl cert must be created and configured with IPSAN to avoid in breaking tls verification (metrics-server etc.)

# Target
- new node will make a csr request with IPSANs or custom cert etc... and approved by kubernetes CA

# How to
- This api will run on master, and uses apiserver-kubelet-client cert/key to authentication/authorization with api server
- On the new node worker with cfssl installed
  + 
  ```
	cat <<EOF | cfssl genkey - | cfssljson -bare server
	{
	  "hosts": [
	    "192.168.33.13"
	  ],
	  "CN": "192.168.33.13",
	  "key": {
	    "algo": "ecdsa",
	    "size": 256
	  }
	}
	EOF
  ```
  +
  ``` 
	cat > server.json << EOF 
	{
		"nodename": "192.168.33.13",
		"nodeip": "192.168.33.13",
		"csrdata": $(cat server.csr) |base64 | tr -d "\n"
	}
	EOF
  ```
  + curl  -H "Content-Type: application/json" -X POST --data @server.json http://127.0.0.1:9999/v1/request > server.crt

- Recheck the cert:
  + openssl x509 -in server.crt -text -noout

- Check the certificate approved or not
  + kubectl get csr

# Build
- install glide
- glide up
- CGO_ENABLED=0 env GOOS=linux go build
