for (( ; ; ))
do
  go run gcePDCreateAttachMount.go

  if [ $? -ne 0 ]
  then
	echo "Test failed: " >&2
	break
  fi
done
