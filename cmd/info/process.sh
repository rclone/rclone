set -euo pipefail

for f in info-*.log; do 
  for pos in middle left right; do
    egrep -oe " Writing $pos position file [^ ]* \w+" $f | sort | cut -d' ' -f 7 > $f.write_$pos
    egrep -oe " Getting $pos position file [^ ]* \w+" $f | sort | cut -d' ' -f 7 > $f.get_$pos
  done
  {
    echo "${${f%.log}#info-}\t${${f%.log}#info-}\t${${f%.log}#info-}\t${${f%.log}#info-}\t${${f%.log}#info-}\t${${f%.log}#info-}"
    echo "Write\tWrite\tWrite\tGet\tGet\tGet"
    echo "Mid\tLeft\tRight\tMid\tLeft\tRight"
    paste $f.write_{middle,left,right} $f.get_{middle,left,right}
  } > $f.csv
done

for f in info-*.list; do
  for pos in middle left right; do
    cat $f | perl -lne 'print $1 if /^\s+[0-9]+\s+(.*)/' | grep -a "position-$pos-" | sort > $f.$pos
  done
  {
    echo "${${f%.list}#info-}\t${${f%.list}#info-}\t${${f%.list}#info-}"
    echo "List\tList\tList"
    echo "Mid\tLeft\tRight"
    for e in 01 02 03 04 05 06 07 08 09 0A 0B 0C 0D 0E 0F 10 11 12 13 14 15 16 17 18 19 1A 1B 1C 1D 1E 1F 20 21 22 23 24 25 26 27 28 29 2A 2B 2C 2D 2E 30 31 32 33 34 35 36 37 38 39 3A 3B 3C 3D 3E 3F 40 41 42 43 44 45 46 47 48 49 4A 4B 4C 4D 4E 4F 50 51 52 53 54 55 56 57 58 59 5A 5B 5C 5D 5E 5F 60 61 62 63 64 65 66 67 68 69 6A 6B 6C 6D 6E 6F 70 71 72 73 74 75 76 77 78 79 7A 7B 7C 7D 7E 7F BF EFBCBC FE; do
      echo -n $(perl -lne 'print "'$e'-$1" if /^position-middle-'$e'-(.*)-/' $f.middle | tr -d "\t\r" | grep -a . || echo Miss)
      echo -n "\t"
      echo -n $(perl -lne 'print "'$e'-$1" if /^(.*)-position-left-'$e'/'    $f.left   | tr -d "\t\r" | grep -a . || echo Miss)
      echo -n "\t"
      echo    $(perl -lne 'print "'$e'-$1" if /^position-right-'$e'-(.*)/'   $f.right  | tr -d "\t\r" | grep -a . || echo Miss)
      # echo -n $(grep -a "position-middle-$e-" $f.middle | tr -d "\t\r" || echo Miss)"\t"
      # echo -n $(grep -a "position-left-$e"    $f.left   | tr -d "\t\r" || echo Miss)"\t"
      # echo    $(grep -a "position-right-$e-"  $f.right  | tr -d "\t\r" || echo Miss)
    done
    } > $f.csv
done

for f in info-*.list; do 
    paste ${f%.list}.log.csv $f.csv > ${f%.list}.full.csv
done
paste *.full.csv > info-complete.csv
