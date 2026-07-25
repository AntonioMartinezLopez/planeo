#!/usr/bin/env bash
set -e

if [ ! -f "$KAFKA_LOG_DIRS/meta.properties" ]; then
  FIRST_BOOT=true
else
  FIRST_BOOT=false
fi

# Generates /opt/kafka/config/server.properties from the KAFKA_* env vars
# (same step the image's default /etc/kafka/docker/run performs) and, as a
# side effect, formats KRaft storage without any SCRAM credential if this
# is the first boot. That side effect is unavoidable: this wrapper doesn't
# expose a way to inject --add-scram into its own format call.
#
# On any boot after the first, this same setup step still tries to format
# storage and exits non-zero because the directory is already formatted.
# The stock image's own /etc/kafka/docker/launch tolerates that exact
# failure (grep for "already formatted" and continue); under `set -e` we
# have to do the same tolerance explicitly, or every restart dies here
# before ever reaching kafka-server-start.sh.
result=$(/opt/kafka/bin/kafka-run-class.sh kafka.docker.KafkaDockerWrapper setup \
  --default-configs-dir /etc/kafka/docker \
  --mounted-configs-dir /mnt/shared/config \
  --final-configs-dir /opt/kafka/config 2>&1) \
  || echo "$result" | grep -iq "already formatted" \
  || { echo "$result"; exit 1; }
echo "$result"

if [ "$FIRST_BOOT" = "true" ]; then
  echo "===> First boot: seeding bootstrap SCRAM admin credential"
  rm -rf "${KAFKA_LOG_DIRS:?}"/*
  /opt/kafka/bin/kafka-storage.sh format \
    --add-scram "SCRAM-SHA-256=[name=admin,password=${KAFKA_BOOTSTRAP_ADMIN_PASSWORD}]" \
    -t "$CLUSTER_ID" \
    -c /opt/kafka/config/server.properties
fi

exec /opt/kafka/bin/kafka-server-start.sh /opt/kafka/config/server.properties
