"""Author the fixed TOKEN Tower campaign. Developer-time only; no runtime generator."""
import json, random, hashlib
from pathlib import Path
SIZE=14
NAMES=[
'入塔长阶','回声石廊','双钥回路','失落军械库','三岔庭院','守夜人的桥','密锁书库','旧城暗道','封印前厅','石堡典狱长',
'余烬阶梯','熔炉回环','铸剑工坊','燃烧栈桥','煤灰仓库','火种岔路','熔岩哨站','赤铜宝库','焦土祭坛','熔炉领主',
'霜雪门廊','冰镜迷途','冻河渡口','寒晶藏室','极夜回廊','双翼冰桥','白塔图书馆','永冻宝库','霜卫王庭','冰原女王',
'暮影入口','黑曜长廊','魔像工场','幽光岔道','沉默钟楼','暗影回环','禁术书库','星尘密室','深渊哨所','暮影审判者',
'王冠之路','三色迷宫','最后的军械库','龙骨回廊','王城密道','破晓前庭','星钥宝库','守誓者之桥','终焉门厅','王冠封印',
'沙海入口','流砂回廊','遗迹祭场','双塔砂桥','沉沙军械库','金风回环','古王书库','琥珀宝库','沙皇前庭','沙海执政官',
'雷鸣台阶','风暴回廊','闪电工坊','悬空长桥','天穹军械库','暴风岔路','雷纹图书馆','风眼宝库','天罚门厅','风暴裁决者',
'冥河入口','亡者回声','灵灯暗道','渡魂长桥','遗忘仓库','幽冥回环','禁魂书库','冥晶宝库','冥王前庭','冥河摆渡者',
'星界之门','银河长廊','星核工坊','彗光桥梁','星舰军械库','轨道回环','观星书库','群星宝库','天幕门厅','星界守望者',
'永夜阶梯','终焉回廊','誓约军械库','时光断桥','命运暗道','黎明回环','创世书库','日冕宝库','最后的封印','百层与黎明']
EN=['The Long Stair','Echo Hall','Twin Key Circuit','Lost Armory','Forked Court','Watchman Bridge','Locked Archive','Old City Passage','Seal Antechamber','Keep Warden','Ember Stair','Furnace Loop','Swordsmith Workshop','Burning Bridge','Ash Warehouse','Fireseed Fork','Lava Outpost','Copper Treasury','Scorched Altar','Furnace Lord','Snow Gate','Ice Mirror Maze','Frozen Crossing','Crystal Chamber','Polar Night Hall','Twin Ice Bridge','White Tower Library','Frozen Treasury','Frost Court','Ice Queen','Twilight Gate','Obsidian Hall','Golem Workshop','Ghostlight Fork','Silent Belfry','Shadow Circuit','Forbidden Archive','Stardust Chamber','Abyss Outpost','Shadow Judge','Crown Road','Three Key Maze','Last Armory','Dragonbone Hall','Royal Passage','Dawn Court','Star Key Treasury','Oathkeeper Bridge','Final Antechamber','Crown Seal',
'Sandsea Gate','Quicksand Hall','Ruined Altar','Twin Sand Bridge','Buried Armory','Golden Wind Loop','Ancient Archive','Amber Treasury','Sand Court','Sandsea Consul',
'Thunder Stair','Storm Hall','Lightning Workshop','Sky Bridge','Celestial Armory','Gale Fork','Thunder Archive','Storm Eye Treasury','Judgment Gate','Storm Arbiter',
'Nether Gate','Echoes of the Dead','Spiritlight Passage','Soul Ferry Bridge','Forgotten Warehouse','Nether Circuit','Soul Archive','Nethercrystal Treasury','Underworld Court','Nether Ferryman',
'Astral Gate','Galaxy Hall','Starcore Workshop','Comet Bridge','Astral Armory','Orbital Circuit','Stargazer Archive','Stellar Treasury','Firmament Gate','Astral Watcher',
'Evernight Stair','Endtime Hall','Oath Armory','Timebroken Bridge','Fate Passage','Dawn Circuit','Genesis Archive','Corona Treasury','Final Seal','One Hundred Floors and Dawn']
BASE=[(65,30,10),(180,110,65),(430,250,180),(800,520,380),(1400,820,700),(1800,1000,820),(2500,1400,1180),(3300,1850,1580),(4200,2400,2050),(5300,3000,2600)]
BOSSES=[(280,90,50),(650,280,180),(1250,550,335),(2100,950,700),(3300,1400,850),(4200,1750,1150),(5400,2250,1550),(6800,2850,2000),(8400,3500,2500),(10500,4250,3050)]
monsters=[]
for z,(hp,atk,defn) in enumerate(BASE):
 for kind,(name,sprite,ability,hm,am,dm) in enumerate([
 ('slime',11,'normal',1,1,1),('bat',12,'double',.65,.8,.65),('skeleton',13,'first',1.1,1.1,.9),('guard',14,'normal',1.7,.95,1.2),('mage',18,'magic',.7,.25,.7),('beast',20,'normal',1.5,1.3,.8),('guardian',22,'normal',2,1.25,1.1),('boss',23,'first',1,1,1)]):
  vals=BOSSES[z] if kind==7 else (round(hp*hm),round(atk*am),round(defn*dm))
  monsters.append(dict(id=200+z*8+kind,hp=vals[0],attack=vals[1],defense=vals[2],gold=(12+kind*3)*(z+1) if kind<7 else 150*(z+1),experience=(3+kind)*(z+1),miss=5 if kind!=1 else 15,criticalThreshold=96,sprite=sprite,name=name,ability=ability))
def bfs(m,start,end):
 q=[start];prev={start:None}
 for p in q:
  if p==end:break
  for dx,dy in [(1,0),(0,-1),(-1,0),(0,1)]:
   n=(p[0]+dx,p[1]+dy)
   if 0<=n[0]<14 and 0<=n[1]<14 and m[n[1]*14+n[0]]==0 and n not in prev:prev[n]=p;q.append(n)
 route=[];p=end
 while p is not None:route.append(p);p=prev[p]
 return route[::-1]
floors=[];rules=[]
for f in range(100):
 rng=random.Random(50127+f*7919);z=f//10;m=[2]*196
 start=(1,11);end=(11,1);stack=[start];visited={start};m[start[1]*14+start[0]]=0
 while stack:
  p=stack[-1];opts=[(p[0]+dx,p[1]+dy) for dx,dy in [(2,0),(-2,0),(0,2),(0,-2)] if 1<=p[0]+dx<=11 and 1<=p[1]+dy<=11 and (p[0]+dx,p[1]+dy) not in visited]
  if not opts:stack.pop();continue
  n=rng.choice(opts);visited.add(n);stack.append(n);m[n[1]*14+n[0]]=0;m[((n[1]+p[1])//2)*14+(n[0]+p[0])//2]=0
 # Selected extra connections give loops and alternative approaches on varied floors.
 for _ in range(f%4):
  walls=[(x,y) for y in range(1,12) for x in range(1,12) if m[y*14+x]==2 and ((x%2==0 and y%2==1) or (y%2==0 and x%2==1))]
  if walls:x,y=rng.choice(walls);m[y*14+x]=0
 path=bfs(m,start,end);pkeys=set(path)
 def put(p,t):m[p[1]*14+p[0]]=t
 # Progression landmarks are separated along the main route, never shuffled at runtime.
 slots=[path[i] for i in sorted(set([1,2,3,len(path)//4,len(path)//3,len(path)//2,2*len(path)//3,3*len(path)//4,len(path)-4,len(path)-2])) if 0<i<len(path)-1]
 tiles=[35 if f==0 else 29,6,32 if f%2==0 else 34,200+z*8,3,30,200+z*8+(f%6),7 if f%2==0 else 28,8 if f%10==8 else 6,200+z*8+(7 if f%10==9 else min(6,1+f%6))]
 for p,t in zip(slots,tiles):put(p,t)
 # Explicit ten-floor seals: the guardian must fall even if a loop bypasses it.
 guardian=path[-2] if f%10==9 else None
 if guardian:put(guardian,200+z*8+7)
 # Side-room vaults, equipment and shrine stations.
 leaves=[(x,y) for y in range(1,12) for x in range(1,12) if (x,y) not in pkeys and m[y*14+x]==0 and sum(m[(y+dy)*14+x+dx]==0 for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)])==1]
 rng.shuffle(leaves)
 for i,p in enumerate(leaves):
  if i==0:tile=31 if f%10==3 else 33 if f%10==6 else 32
  elif i==1:tile=34
  elif i==2:tile=51 if f%10 in [2,6] else 30
  elif i==3:tile=30
  else:tile=[28,29,6,200+z*8+(f%5)][i%4]
  put(p,tile)
  # Vault entrance can be revisited after finding a key upstairs.
  if i==0 and f%5==1:
   for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]:
    n=(p[0]+dx,p[1]+dy)
    if m[n[1]*14+n[0]]==0 and n not in pkeys:put(n,4);break
  if i==1 and f%5==3:
   for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]:
    n=(p[0]+dx,p[1]+dy)
    if m[n[1]*14+n[0]]==0 and n not in pkeys:put(n,5);break
 # Every area has an accessible shrine; if there are few leaves reserve a branch cell.
 if f%10 in [2,6] and 51 not in m:
  branch=next((p for p in visited if p not in pkeys and m[p[1]*14+p[0]]==0),None)
  if branch:put(branch,51)
 put(start,0 if f==0 else 90);put(end,99 if f==99 else 80)
 # A shrine is interacted with from beside it. It must never cut off treasure.
 def walkable():
  reached={start};queue=[start]
  for x,y in queue:
   for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)]:
    n=(x+dx,y+dy)
    if 0<=n[0]<14 and 0<=n[1]<14 and n not in reached and m[n[1]*14+n[0]] not in [2,51]:reached.add(n);queue.append(n)
  return reached
 if any(t not in [0,2,51] and (i%14,i//14) not in walkable() for i,t in enumerate(m)):
  shrine=m.index(51);m[shrine]=0
  alcove=next((x,y) for y in range(1,12) for x in range(1,12) if m[y*14+x]==2 and sum(m[(y+dy)*14+x+dx]!=2 for dx,dy in [(1,0),(-1,0),(0,1),(0,-1)])==1)
  put(alcove,51)
 assert all(t in [0,2,51] or (i%14,i//14) in walkable() for i,t in enumerate(m))
 floors.append(m);rules.append(dict(zone=z,**({'guardian':dict(x=guardian[0],y=guardian[1])} if guardian else {}),name=NAMES[f],nameEn=EN[f],layout=f%10))
assert len({tuple(f) for f in floors})==100
out=dict(sourceRevision='token-tower-100-v1',long=True,start=dict(x=1,y=11),floors=floors,monsters=monsters,floorRules=rules)
Path(__file__).with_name('campaign.json').write_text(json.dumps(out,separators=(',',':'))+'\n')
print(json.dumps(dict(floors=len(floors),monsters=len(monsters),bytes=Path(__file__).with_name('campaign.json').stat().st_size,hash=hashlib.sha256(json.dumps(out).encode()).hexdigest())))
