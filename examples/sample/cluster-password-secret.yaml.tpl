apiVersion: v1
kind: Secret
metadata:
  name: cluster-password
stringData:
  # It must have at least 11 characters
  password: <your-password-here>
