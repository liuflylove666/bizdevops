#!/bin/bash
set -euo pipefail

for _ in $(seq 1 120); do
  if gitlab-rails runner "ApplicationSetting.current" >/dev/null 2>&1; then
    break
  fi
  sleep 5
done

gitlab-rails runner "
settings = Gitlab::CurrentSettings.current_application_settings
changed = false

if settings.ci_jwt_signing_key.blank?
  settings.ci_jwt_signing_key = OpenSSL::PKey::RSA.new(2048).to_pem
  changed = true
end

if settings.ci_job_token_signing_key.blank?
  settings.ci_job_token_signing_key = OpenSSL::PKey::RSA.new(2048).to_pem
  changed = true
end

token = ENV['GITLAB_SHARED_RUNNERS_REGISTRATION_TOKEN'].presence || ENV['GITLAB_RUNNER_REGISTRATION_TOKEN'].presence
if token.present?
  settings.set_runners_registration_token(token)
  changed = true
end

settings.save! if changed
puts 'GitLab CI signing keys and runner registration token are ready'
"
