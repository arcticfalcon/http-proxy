FROM fluent/fluentd:stable-debian

WORKDIR /home/fluent
# ENV PATH /home/fluent/.gem/ruby/2.4.0/bin:$PATH
ENV APT_INS="build-essential make libcurl4-gnutls-dev"
ENV APT_DEL="build-essential make libcurl4-gnutls-dev"
ARG GEM_NAME="fluent-plugin-elasticsearch"

USER root
RUN apt-get update && \
      apt-get install -y ${APT_INS} && \
      gem install ${GEM_NAME} && \
      gem sources --clear-all && \
      apt-get remove -y ${APT_DEL} && \
      apt-get autoremove -y && \
      rm /var/lib/gems/*/cache/* && \
      rm -rf /var/lib/apt/lists/*

# Add fluent user as the fluent image doesnt have for some reason
RUN groupadd -r fluent && useradd -r -g fluent fluent || \
      chown -R fluent /fluentd && chgrp -R fluent /fluentd

USER fluent

EXPOSE 24224

ENTRYPOINT exec fluentd -c /fluentd/etc/fluent.conf -p /fluentd/plugins