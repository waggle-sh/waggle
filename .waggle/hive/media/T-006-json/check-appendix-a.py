import json,re,sys
t=open('.waggle/hive/tasks/T-006-backfill.md').read()
a=t.split('## Appendix A')[1].split('## Appendix B')[0]
items=re.split(r'\n(?=\*\*D-\d{3}\*\* \|)',a)[1:]
bad=0;n=0
for it in items:
    L=it.strip().split('\n')
    m=re.match(r'\*\*(D-\d{3})\*\* \| kind `(\w+)` \| tickets (.*)',L[0]);id,k,tk=m.groups()
    f={}
    for l in L[1:]:
        m=re.match(r'- (\w+): (.*)',l)
        if m: f[m[1]]=m[2]
    j=json.load(open(f'internal/decisions/testdata/backfill/{id}.json'))
    exp=dict(kind=k,tickets=[] if tk=='none' else [x.strip() for x in tk.strip('`').split(',')],title=f['Title'],question=f['Question'],options=f['Options'].split(' || '),recommendation=f['Recommendation'],choice=f['Choice'],rationale=f['Rationale'],supersedes=[x.strip() for x in f['Supersedes'].split(',')] if 'Supersedes' in f else [])
    n+=1
    if j!=exp: bad+=1;print(id,'DIFF',{k:(j.get(k),exp[k]) for k in exp if j.get(k)!=exp[k]})
print(n,'items',bad,'bad')
