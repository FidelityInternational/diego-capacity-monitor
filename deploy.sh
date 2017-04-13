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
  cf push "${APP_NAME:-diego-capacity-monitor}" --no-start
  setEnvs "${APP_NAME:-diego-capacity-monitor}"
  cf bind-service "${APP_NAME:-diego-capacity-monitor}" cache || true
  cf start "${APP_NAME:-diego-capacity-monitor}"
else
  echo "Zero downtime deploying..."
  domain=$(cf app "${APP_NAME:-diego-capacity-monitor}" | grep urls | cut -d":" -f2 | xargs | cut -d"." -f 2-)
  cf push "${APP_NAME:-diego-capacity-monitor}-green" -f manifest.yml -n "${APP_NAME:-diego-capacity-monitor}-green" --no-start
  setEnvs "${APP_NAME:-diego-capacity-monitor}-green"
  cf bind-service "${APP_NAME:-diego-capacity-monitor}-green" cache || true
  cf start "${APP_NAME:-diego-capacity-monitor}-green"
  cf map-route "${APP_NAME:-diego-capacity-monitor}-green" "${domain}" -n "${APP_NAME:-diego-capacity-monitor}"
  cf delete "${APP_NAME:-diego-capacity-monitor}" -f
  cf rename "${APP_NAME:-diego-capacity-monitor}-green" "${APP_NAME:-diego-capacity-monitor}"
  cf unmap-route "${APP_NAME:-diego-capacity-monitor}" "${domain}" -n "${APP_NAME:-diego-capacity-monitor}-green"
fi
