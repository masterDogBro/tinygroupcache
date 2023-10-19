# 这条指令在git bash执行并不能杀掉server，后续测试需要注意
trap "rm server;kill 0" EXIT

go build -o server
./server -port=8001 &
./server -port=8002 &
./server -port=8003 -api=1 &

sleep 2
echo ">>> start test"

# 这样测试并不能完全并发，我使用自己的笔记本编程，当我电脑上的下载软件关闭后，电脑负载很低，singlefight没有生效的条件了
for ((i=1; i<=3; i++))
do
  curl "http://localhost:9999/api?key=Tom"
done

wait