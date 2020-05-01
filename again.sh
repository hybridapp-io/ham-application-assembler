oc project ham
kubectl delete -f deploy/
operator-sdk build $IMAGE
docker push $IMAGE
kubectl apply -f deploy
oc project default
sleep 20s
kubectl logs `kubectl get pods -n ham | grep ham-application-assembler | head -n1 | awk '{print $1;}'` -n ham -f
