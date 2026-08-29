ARG NETINIT_BASE_IMAGE=rancher/k3s:v1.35.5-k3s1
FROM ${NETINIT_BASE_IMAGE}
RUN mkdir -p /usr/local/bin && \
    ln -sf /bin/aux/iptables /usr/local/bin/iptables && \
    ln -sf /bin/aux/ip6tables /usr/local/bin/ip6tables
