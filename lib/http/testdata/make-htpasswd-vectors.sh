#!/bin/bash
# Prints label<TAB>secret<TAB>password lines made by real htpasswd
# tools, used to make htpasswd-golden.tsv which adds variations of the
# password and whether each should be accepted.
pws=("" "a" "password" "md5" "exactly16chars!!" "seventeen_chars!!" "thirty-three-characters-long-pass" "$(printf 'x%.0s' {1..80})" "pässwörd" "has:colon" "has space" "日本語パスワード" 'dol$lar' '#hash')
out() { printf '%s\t%s\t%s\n' "$1" "$2" "$3"; }
for pw in "${pws[@]}"; do
  out htpasswd-m "$(htpasswd -nbm u "$pw" | cut -d: -f2-)" "$pw"
  out htpasswd-B4 "$(htpasswd -nbB -C 4 u "$pw" | cut -d: -f2-)" "$pw"
  out htpasswd-s "$(htpasswd -nbs u "$pw" | cut -d: -f2-)" "$pw"
  out htpasswd-d "$(htpasswd -nbd u "$pw" 2>/dev/null | cut -d: -f2-)" "$pw"
  out htpasswd-p "$(htpasswd -nbp u "$pw" 2>/dev/null | cut -d: -f2-)" "$pw"
  out openssl-1 "$(openssl passwd -1 "$pw")" "$pw"
  out openssl-apr1 "$(openssl passwd -apr1 "$pw")" "$pw"
  out openssl-1-shortsalt "$(openssl passwd -1 -salt ab "$pw")" "$pw"
  out openssl-apr1-emptysalt "$(openssl passwd -apr1 -salt '' "$pw")" "$pw"
  out openssl-5 "$(openssl passwd -5 "$pw")" "$pw"
  out openssl-6 "$(openssl passwd -6 "$pw")" "$pw"
  [ -n "$pw" ] && out mkpasswd-md5 "$(mkpasswd -m md5crypt "$pw")" "$pw"
  for p in 2a 2b; do
    out python-$p "$(python3 -I -c 'import bcrypt,sys; print(bcrypt.hashpw(sys.argv[1].encode(), bcrypt.gensalt(4, prefix=sys.argv[2].encode())).decode())' "$pw" $p)" "$pw"
  done
  h=$(python3 -I -c 'import bcrypt,sys; print(bcrypt.hashpw(sys.argv[1].encode(), bcrypt.gensalt(4)).decode())' "$pw")
  out bcrypt-2x "\$2x\$${h:4}" "$pw"
  out bcrypt-2 "\$2\$${h:4}" "$pw"
done
out htpasswd-B10 "$(htpasswd -nbB -C 10 u password | cut -d: -f2-)" password
