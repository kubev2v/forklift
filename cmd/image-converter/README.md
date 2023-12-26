# Image Converter

Convert the format of images. Since KubeVirt requires RAW images, we sometimes have to convert images before attaching them to a VM.

## Measuring

1. Upload cirros PVC

```shell
# Download cirros image
$ curl -LO https://download.cirros-cloud.net/0.6.2/cirros-0.6.2-x86_64-disk.img

# Create PVC with above image
$ virtctl image-upload pvc cirros --size=1Gi --image-path=cirros-0.6.2-x86_64-disk.img -n default --storage-class nfs-csi --insecure
```

# TODO: remove once this is sorted out
2. Create service account in target namespace

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: forklift-populator-controller
  namespace: default
```

# TODO: create a separate account?

3. Create converter pod job

```yaml
echo 'apiVersion: batch/v1
kind: Job
metadata:
  name: measure-pvc
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: measure-pvc
        image: default-route-openshift-image-registry.apps.zmeya.rh-internal.com/openshift/image-converter:latest
        args:
          - "-command"
          - "measure"
          - "-src-path"
          - "/mnt/disk.img"
          - "-pvc-name"
          - "cirros"
          - "-namespace"
          - "default"
          - "-target-format"
          - "raw"
          - "-src-format"
          - "qcow2"
        volumeMounts:
        - name: cirros
          mountPath: /mnt
      restartPolicy: Never
      volumes:
      - name: cirros
        persistentVolumeClaim:
          claimName: cirros
      serviceAccountName: forklift-populator-controller' | kubectl apply -f -
```

4. Check the result

```shell
$ kubectl get pvc mypvc -n default -o jsonpath='forklift.konveyor.io/required-size: {.metadata.annotations.forklift\.konveyor\.io/required-size}'
```

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
        image: default-route-openshift-image-registry.apps.zmeya.rh-internal.com/openshift/image-converter:latest
        args:
          - "-command"
          - "convert"
          - "-src-path"
          - "/mnt/disk.img"
          - "-dst-path"
          - "/output/disk.img"
          - "-target-format"
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