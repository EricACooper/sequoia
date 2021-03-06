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
ARG BRANCH=master
RUN git checkout $BRANCH

# install python deps
RUN pip2 install --upgrade packaging appdirs
RUN pip install -U pip setuptools
RUN pip install paramiko &&\
    pip install gevent &&\
    pip install pyyaml &&\
    pip install ntplib &&\
    pip install couchbase &&\
    pip install decorator &&\
    pip install ecdsa &&\
    pip install Fabric &&\
    pip install iniparse &&\
    pip install mercurial &&\
    pip install paramiko &&\
    pip install httplib2 &&\
    pip install boto &&\
    pip install futures &&\
    pip install greenlet &&\
    pip install urllib3 &&\
    pip install btrc

COPY local.ini local.ini
COPY upgrade_local.ini upgrade_local.ini
COPY host2ip.sh host2ip.sh
COPY testrunner testrunner
RUN sed -i 's/IS_CONTAINER.*/IS_CONTAINER = True/' lib/testconstants.py
ENTRYPOINT ["./testrunner"]
