anitui add title -d 03-02-2006

anitui (default action: list entries) -> (in fzf)
 -s status    e.g anitui -s finished (only show titles with status 'finished')
 -emitStatus (def: true) also print status in list
 -sort [[sortBy]] (def: )
 -dateFormat: go date format

anitui [[title]] -> print details: status, progress, watches
anitui [[title]] -s status -> change status
anitui [[title]] -d -> delete title
anitui [[title]] -p [[int]] -> set progress (episode) to argument (can be negative)
anitui [[title]] -r [[float]] -> set local rating
anitui [[title]] -n [[string]] -> set notes
anitui [[title]] how to list watches, set/remove dates and statuses for them ??????? use determenistic indexes?  -w [[date]] -> set watch date/dates
anitui [[title]] -u -> add today as last watch entry, increment progress by 1
