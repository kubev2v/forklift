# Image Converter

Convert the format of images. Since KubeVirt requires RAW images, we sometimes have to convert images before attaching them to a VM.

## Converting

1. Create an empty PVC with the measured size

```yaml
echo 'apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: scratch-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1014366208
  storageClassName: nfs-csi' | kubectl apply -f -
```

2. Create converter pod job

```yaml
echo 'apiVersion: batch/v1
kind: Job
metadata:
  name: convert-pvc
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: convert-pvc
        image: quay.io/bzlotnik/image-converter:latest
        args:
          - "-src-path"
          - "/mnt/disk.img"
          - "-dst-path"
          - "/output/disk.img"
          - "-dst-format"
          - "qcow2"
          - "-src-format"
          - "raw"
        volumeMounts:
        - name: cirros
          mountPath: /mnt
        - name: scratch
          mountPath: /output
      restartPolicy: Never
      volumes:
      - name: cirros
        persistentVolumeClaim:
          claimName: cirros
      - name: scratch
        persistentVolumeClaim:
          claimName: scratch-pvc
      serviceAccountName: forklift-populator-controller' | kubectl apply -f -
```


NOTE: `virtctl upload-image` converts the image to RAW, so we the examples converts raw -> qcow2