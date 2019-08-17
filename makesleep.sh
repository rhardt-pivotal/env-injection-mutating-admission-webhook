cat <<EOF | kubectl create -f -
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: sleep
  namespace: demo
spec:
  replicas: 1
  template:
    metadata:
#      annotations:
#        env-injector-webhook.hardt.io/inject: "yes"
      labels:
        app: sleep
    spec:
      containers:
      - name: sleep
        image: tutum/curl
        command: ["/bin/sleep","infinity"]
        imagePullPolicy: 
        env: 
        - name: rob_y
          value: bing
EOF