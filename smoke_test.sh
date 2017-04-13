#!/bin/sh -e

begin() {
  echo "----------------------Begin Smoke Tests--------------------"
  echo "${APP_URL}"
}

end() {
  echo "------------------------End Smoke Tests--------------------"
}

fail() {
  printf " ... FAIL: %s" "$1"
  end
  exit 1
}

pass() {
   printf " ... PASS\n"
}

test_home_page_is_ok() {
  response="$(curl --max-time 10 --connect-timeout 3 -svLk "${APP_URL}")"
  printf "Response: %s\n" "${response}"

  status=$(echo "${response}" | awk "/.+healthy.+message.+/")

  if [ -z "$status" ]; then
    fail "${response} did not contain healthy and message"
  else
    pass
  fi
}

begin
test_home_page_is_ok
end
