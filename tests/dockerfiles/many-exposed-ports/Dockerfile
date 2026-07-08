FROM alpine:3.14.2

# 25 exposed TCP ports, used to test that the docker executor's exposed-port
# healthcheck logic deterministically keeps only the lowest maxPortsCheck (20)
# port numbers when a service declares more than that.
EXPOSE 1000/tcp
EXPOSE 1001/tcp
EXPOSE 1002/tcp
EXPOSE 1003/tcp
EXPOSE 1004/tcp
EXPOSE 1005/tcp
EXPOSE 1006/tcp
EXPOSE 1007/tcp
EXPOSE 1008/tcp
EXPOSE 1009/tcp
EXPOSE 1010/tcp
EXPOSE 1011/tcp
EXPOSE 1012/tcp
EXPOSE 1013/tcp
EXPOSE 1014/tcp
EXPOSE 1015/tcp
EXPOSE 1016/tcp
EXPOSE 1017/tcp
EXPOSE 1018/tcp
EXPOSE 1019/tcp
EXPOSE 1020/tcp
EXPOSE 1021/tcp
EXPOSE 1022/tcp
EXPOSE 1023/tcp
EXPOSE 1024/tcp

CMD ["sleep", "3600"]
