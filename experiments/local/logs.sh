echo "translate logs to human readable format"
for x in $(ls *.log);
do
    echo $x
    go run ../../cmd/log4humanz/main.go $x > $x"h"
done

echo "analyze logs"
python3 ../../log_analyzer/loganal.py . > 'summary'

for x in $(ls *.log);
do
    echo $x
    python3 ../../log_analyzer/loganal.py $x > $x".out"
done

