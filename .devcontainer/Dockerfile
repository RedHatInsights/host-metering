FROM registry.access.redhat.com/ubi7

RUN yum install go-toolset-1.19 git make curl gzip tar --enablerepo=rhel-7-server-devtools-rpms -y

COPY ./entrypoint.sh /usr/bin/
RUN chmod +x /usr/bin/entrypoint.sh

COPY ./enable_go_scl.sh /etc/profile.d/
RUN chmod +x /etc/profile.d/enable_go_scl.sh

ENTRYPOINT ["entrypoint.sh"]
