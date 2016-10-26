FROM ubuntu:14.04
RUN apt-get update
RUN apt-get install -y gcc g++ make cmake git-core libevent-dev libev-dev libssl-dev libffi-dev psmisc iptables zip unzip python-dev python-pip ntp

# build libcouchbase
RUN git clone git://github.com/couchbase/libcouchbase.git && \
    mkdir libcouchbase/build

WORKDIR libcouchbase/build
RUN ../cmake/configure --prefix=/usr && \
      make && \
      make install

WORKDIR /
RUN git clone git://github.com/couchbase/testrunner.git
WORKDIR testrunner

# install python deps
RUN pip install paramiko &&\
    pip install gevent &&\
    pip install boto &&\
    pip install pyyaml &&\
    pip install ntplib &&\
    pip install httplib2 &&\
    pip install decorator &&\
    pip install ecdsa &&\
    pip install fabric &&\
    pip install iniparse &&\
    pip install mercurial &&\
    pip install pygpgme &&\
    pip install pycrypto &&\
    pip install pycurl &&\
    pip install urlgrabber &&\
    pip install yum-metadata-parser  &&\
    pip install futures &&\
    pip install gevent &&\
    pip install greenlet &&\
    pip install urllib3 &&\
    pip install btrc &&\
    pip install couchbase

COPY local.ini local.ini
COPY upgrade_local.ini upgrade_local.ini
COPY host2ip.sh host2ip.sh
COPY testrunner testrunner
ENTRYPOINT ["./testrunner"]
