#!/bin/bash

set -e

setEnvs () {
  cf set-env "$1" CF_USERNAME "${CF_USERNAME}"
  cf set-env "$1" CF_PASSWORD "${CF_PASSWORD}"
  cf set-env "$1" CF_API_ENDPOINT "${CF_API_ENDPOINT}"
  cf set-env "$1" WATERMARK "${WATERMARK}"
}

echo "Logging into CF..."
cf api https://api."${CF_SYS_DOMAIN}" --skip-ssl-validation
cf auth "${CF_DEPLOY_USERNAME}" "${CF_DEPLOY_PASSWORD}"
echo "Targeting Org and Space..."
cf target -o "${ORG_NAME:-diego-capacity-monitor}" -s "${SPACE_NAME:-diego-capacity-monitor}"
echo "Deploying apps..."
echo "Setting up services..."
if [[ "$(cf service cache || true)" == *"FAILED"* ]] ; then
  cf create-service p-redis shared-vm cache || true
fi
if [[ "$(cf app "${APP_NAME:-diego-capacity-monitor}") || true)" == *"FAILED"* ]] ; then
  cf push "${APP_NAME:-diego-capacity-monitor}" --no-start -s "${STACK:-cflinuxfs3}"
  setEnvs "${APP_NAME:-diego-capacity-monitor}"
  cf bind-service "${APP_NAME:-diego-capacity-monitor}" cache || true
  cf start "${APP_NAME:-diego-capacity-monitor}"
else
  echo "Zero downtime deploying..."
  cf push "${APP_NAME:-diego-capacity-monitor}" -f manifest.yml --strategy rolling
fi
