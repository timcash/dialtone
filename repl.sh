#!/usr/bin/env bash
set -euo pipefail

printf "DIALTONE> What is your name?\n"
printf "USER-1> "
IFS= read -r user_name || exit 0
user_name="${user_name:-USER-1}"
printf "DIALTONE> Hello, %s.\n" "$user_name"
printf "DIALTONE> Quiz time. I will ask 3 questions in random order.\n"

questions=(
  "What is 2 + 2?"
  "What color is the sky on a clear day?"
  "What is the first letter of the English alphabet?"
)

answers=(
  "4"
  "blue"
  "a"
)

order=(0 1 2)
for ((i=2; i>0; i--)); do
  j=$((RANDOM % (i + 1)))
  tmp="${order[i]}"
  order[i]="${order[j]}"
  order[j]="$tmp"
done

score=0
for idx in "${order[@]}"; do
  printf "DIALTONE> %s\n" "${questions[idx]}"
  printf "USER-1> "
  if ! IFS= read -r user_input; then
    printf "\nDIALTONE> Session closed.\n"
    exit 0
  fi

  cleaned_input="$(printf "%s" "$user_input" | tr '[:upper:]' '[:lower:]' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
  expected="${answers[idx]}"

  if [ "$cleaned_input" = "$expected" ]; then
    score=$((score + 1))
    printf "DIALTONE> Correct.\n"
  else
    printf "DIALTONE> Not correct. Moving to the next question.\n"
  fi
done

printf "DIALTONE> Quiz complete, %s. Final score: %d/3.\n" "$user_name" "$score"
