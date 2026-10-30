import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { CalendarDays, CalendarPlus, CheckCircle2, ChevronLeft, ChevronRight, CircleDashed, Clock3, Link2, Plus, Sparkles } from 'lucide-react';
import { useSearchParams } from 'react-router';
import { api } from '@/lib/api';
import type { Issue, Plan, Repository } from '@/types/idios';
import { Badge, Button, Card, EmptyState, Field, PageHeading } from '@/design-system';
import { Modal } from '@/design-system';
import { PlanDetailsModal } from '@/features/plans';

const dateKey = (date: Date) => `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}`;
const shiftDate = (key: string, days: number) => { const date = new Date(`${key}T12:00:00`); date.setDate(date.getDate()+days); return dateKey(date); };
const todayKey = () => dateKey(new Date());

type PlanDraft = { title: string; description: string; plan_date: string; linked_entity_type: string; linked_entity_id: string; tasks: string };
const emptyDraft = (date: string): PlanDraft => ({ title: '', description: '', plan_date: date, linked_entity_type: '', linked_entity_id: '', tasks: '' });

export function PlansPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedDate, setSelectedDate] = useState(() => searchParams.get('date') || todayKey());
  const [plans, setPlans] = useState<Plan[]>([]);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [selectedPlan, setSelectedPlan] = useState<Plan | null>(null);
  const [summaryPlans, setSummaryPlans] = useState<Plan[] | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<Plan | null>(null);
  const [draft, setDraft] = useState<PlanDraft>(emptyDraft(selectedDate));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const lastAutoOpen = useRef('');

  const load = useCallback(async (quiet = false, baseDate = selectedDate) => {
    if (!quiet) setLoading(true);
    setError('');
    try {
      const [nextPlans, nextRepos, nextIssues] = await Promise.all([
        api.plans({ from: shiftDate(baseDate, -365), to: shiftDate(baseDate, 365) }),
        api.repositories(), api.issues(),
      ]);
      setPlans(nextPlans); setRepositories(nextRepos); setIssues(nextIssues);
      setSelectedPlan((current) => current ? nextPlans.find((item) => item.id === current.id) ?? null : current);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load plans.'); }
    finally { if (!quiet) setLoading(false); }
  }, [selectedDate]);
  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    const fromQuery = searchParams.get('date');
    if (fromQuery && /^\d{4}-\d{2}-\d{2}$/.test(fromQuery) && fromQuery !== selectedDate) setSelectedDate(fromQuery);
  }, [searchParams, selectedDate]);
  useEffect(() => {
    const target = searchParams.get('date');
    if (!target || !plans.length || lastAutoOpen.current === target) return;
    const matches = plans.filter((plan) => plan.plan_date === target);
    if (matches.length === 1) { setSelectedPlan(matches[0]); lastAutoOpen.current = target; }
    else if (matches.length > 1) { setSummaryPlans(matches); lastAutoOpen.current = target; }
  }, [plans, searchParams]);

  const currentPlans = useMemo(() => plans.filter((plan) => plan.plan_date === selectedDate), [plans, selectedDate]);
  const nextPlans = useMemo(() => plans.filter((plan) => plan.plan_date > selectedDate).sort((a,b) => a.plan_date.localeCompare(b.plan_date)).slice(0,5), [plans, selectedDate]);
  const currentTotals = useMemo(() => currentPlans.reduce((acc, plan) => {
    acc.plans++; acc.tasks += plan.tasks.length; acc.done += plan.tasks.filter((task) => task.done).length; return acc;
  }, { plans: 0, tasks: 0, done: 0 }), [currentPlans]);
  const weekDays = useMemo(() => {
    // The planner is a forward-looking seven-day window. Anchor the first card
    // to the selected date (today by default), never to the previous Sunday.
    const date = new Date(`${selectedDate}T12:00:00`);
    return Array.from({ length: 7 }, (_, index) => {
      const day = new Date(date); day.setDate(date.getDate()+index); const key = dateKey(day);
      const dayPlans = plans.filter((plan) => plan.plan_date === key);
      const total = dayPlans.reduce((count, plan) => count + plan.tasks.length, 0);
      const done = dayPlans.reduce((count, plan) => count + plan.tasks.filter((task) => task.done).length, 0);
      return { key, date: day, total, done, count: dayPlans.length };
    });
  }, [selectedDate, plans]);

  function chooseDate(next: string) {
    setSelectedDate(next);
    const params = new URLSearchParams(searchParams); params.delete('date'); setSearchParams(params, { replace: true });
    setSelectedPlan(null); setSummaryPlans(null); lastAutoOpen.current = '';
  }
  function startCreate() { setSummaryPlans(null); setEditingPlan(null); setDraft(emptyDraft(selectedDate)); setEditorOpen(true); }
  function startEdit(plan: Plan) {
    setSelectedPlan(null); setEditingPlan(plan);
    setDraft({ title: plan.title, description: plan.description, plan_date: plan.plan_date, linked_entity_type: plan.linked_entity_type ?? '', linked_entity_id: plan.linked_entity_id ?? '', tasks: '' });
    setEditorOpen(true);
  }
  function updateDraft<K extends keyof PlanDraft>(key: K, value: PlanDraft[K]) {
    setDraft((current) => ({ ...current, [key]: value, ...(key === 'linked_entity_type' ? { linked_entity_id: '' } : {}) }));
  }
  async function savePlan(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (saving) return;
    const taskTitles = draft.tasks.split('\n').map((line) => line.trim()).filter(Boolean);
    if (!editingPlan && taskTitles.length === 0) { setError('Add at least one task to your plan.'); return; }
    setSaving(true); setError('');
    try {
      const payload = { title: draft.title.trim(), description: draft.description.trim(), plan_date: draft.plan_date, linked_entity_type: draft.linked_entity_type, linked_entity_id: draft.linked_entity_id };
      if (editingPlan) {
        const updated = await api.updatePlan(editingPlan.id, payload);
        if (taskTitles.length) for (const title of taskTitles) await api.addPlanTask(updated.id, title);
        setSelectedDate(updated.plan_date); await load(true, updated.plan_date); setSelectedPlan(updated);
      } else {
        const created = await api.createPlan({ ...payload, tasks: taskTitles });
        setSelectedDate(created.plan_date); await load(true, created.plan_date); setSelectedPlan(created);
      }
      setEditorOpen(false); setEditingPlan(null);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to save this plan.'); }
    finally { setSaving(false); }
  }
  async function refreshAfterChange() { await load(true, selectedPlan?.plan_date ?? selectedDate); }
  function deletedPlan(id: string) { setPlans((current) => current.filter((plan) => plan.id !== id)); if (selectedPlan?.id === id) setSelectedPlan(null); }

  return <div className="page-stack plans-page">
    <PageHeading eyebrow="INTENTIONAL WORK" title="Daily plans" description="Give each day a shape. Plans turn intentions into small tasks, linked to the Issues or Repositories they move forward." actions={<><Button variant="secondary" onClick={() => void load()} loading={loading}><Clock3 size={15}/> Refresh</Button><Button onClick={startCreate}><CalendarPlus size={16}/> New plan</Button></>} />
    {error && !editorOpen && <div className="notice notice--error" role="alert">{error}</div>}
    <Card className="plans-hero">
      <div className="plans-hero__copy"><span className="plans-hero__icon"><Sparkles size={18}/></span><div><span className="eyebrow">YOUR DAILY INTENTION</span><h2>{selectedDate === todayKey() ? 'Today, one step at a time.' : new Date(`${selectedDate}T12:00:00`).toLocaleDateString(undefined,{weekday:'long',month:'long',day:'numeric'})}</h2><p>{currentTotals.tasks ? `${currentTotals.done} of ${currentTotals.tasks} tasks completed across ${currentTotals.plans} plan${currentTotals.plans===1?'':'s'}.` : 'Keep it kind and realistic. What would make this day feel meaningful?'}</p></div></div>
      <div className="plans-hero__progress"><div><span>DAILY PROGRESS</span><strong>{currentTotals.tasks ? Math.round(currentTotals.done/currentTotals.tasks*100) : 0}%</strong></div><div className="plan-progress-track"><i style={{width:`${currentTotals.tasks?currentTotals.done/currentTotals.tasks*100:0}%`}}/></div><small>{Math.max(0,currentTotals.tasks-currentTotals.done)} task{currentTotals.tasks-currentTotals.done===1?'':'s'} remaining</small></div>
    </Card>

    <section className="plans-date-panel panel">
      <div className="plans-date-panel__heading"><div><span className="eyebrow">DATE PLANNER</span><h2>Choose a day</h2><p>{selectedDate === todayKey() ? "Today first, followed by the next six days." : "Selected day first, followed by the next six days."}</p></div><div className="plans-date-controls"><Button type="button" variant="secondary" onClick={() => chooseDate(shiftDate(selectedDate,-1))} aria-label="Previous day"><ChevronLeft size={15}/></Button><input aria-label="Select planning date" type="date" value={selectedDate} onChange={(event) => chooseDate(event.target.value)}/><Button type="button" variant="secondary" onClick={() => chooseDate(shiftDate(selectedDate,1))} aria-label="Next day"><ChevronRight size={15}/></Button><Button type="button" variant="ghost" onClick={() => chooseDate(todayKey())}>Today</Button></div></div>
      <div className="plans-week-strip">{weekDays.map((day) => <button type="button" className={`plans-week-day ${day.key===selectedDate?'plans-week-day--selected':''} ${day.key===todayKey()?'plans-week-day--today':''}`} key={day.key} onClick={() => chooseDate(day.key)}><span>{day.date.toLocaleDateString(undefined,{weekday:'short'})}</span><strong>{day.date.getDate()}</strong><small>{day.count ? `${day.count} plan${day.count===1?'':'s'}` : day.total ? `${day.done}/${day.total}` : '—'}</small>{day.total>0&&<i className="plans-week-day__meter"><b style={{width:`${day.done/day.total*100}%`}}/></i>}</button>)}</div>
    </section>

    <div className="plans-workspace-grid">
      <section className="plans-day-section">
        <div className="plans-section-heading"><div><span className="eyebrow">SCHEDULE</span><h2>{selectedDate===todayKey()?'Today’s plans':`Plans for ${new Date(`${selectedDate}T12:00:00`).toLocaleDateString(undefined,{month:'short',day:'numeric'})}`}</h2></div><Badge tone={currentTotals.tasks>0&&currentTotals.done===currentTotals.tasks?'green':'blue'}>{currentPlans.length} plan{currentPlans.length===1?'':'s'}</Badge></div>
        {loading ? <div className="plans-loading">Loading your plans…</div> : currentPlans.length ? <div className="plans-card-list">{currentPlans.map((plan) => {
          const done=plan.tasks.filter((task)=>task.done).length; const percent=plan.tasks.length?Math.round(done/plan.tasks.length*100):0;
          return <button type="button" key={plan.id} className="plan-overview-card" onClick={() => setSelectedPlan(plan)}><span className={`plan-overview-card__status ${percent===100&&plan.tasks.length?'plan-overview-card__status--done':''}`}>{percent===100&&plan.tasks.length?<CheckCircle2 size={19}/>:<CircleDashed size={19}/>}</span><span className="plan-overview-card__main"><span className="plan-overview-card__top"><strong>{plan.title}</strong><span>{done}/{plan.tasks.length}</span></span>{plan.description&&<small className="plan-overview-card__description">{plan.description}</small>}<span className="plan-overview-card__footer">{plan.linked_entity_label&&<em><Link2 size={12}/>{plan.linked_entity_label}</em>}<span className="plan-overview-card__meter"><i style={{width:`${percent}%`}}/></span>{plan.tasks.length?`${percent}%`: 'No tasks'}</span></span><ChevronRight size={16}/></button>;
        })}</div> : <Card className="plans-empty-card"><EmptyState title="No plans for this day" body="Keep the day intentional. Add a plan, break it into small tasks, and link it to the issue or repository it supports." icon={<CalendarDays size={23}/>} /><Button onClick={startCreate}><Plus size={15}/> Create a plan for this day</Button></Card>}
      </section>
      <aside className="plans-upcoming panel"><div className="plans-section-heading"><div><span className="eyebrow">LOOKING AHEAD</span><h2>Coming up</h2></div><CalendarDays size={17}/></div>{nextPlans.length ? nextPlans.map((plan) => <button className="plans-upcoming-row" type="button" key={plan.id} onClick={() => {chooseDate(plan.plan_date); setSelectedPlan(plan);}}><span className="plans-upcoming-row__date"><strong>{new Date(`${plan.plan_date}T12:00:00`).getDate()}</strong><small>{new Date(`${plan.plan_date}T12:00:00`).toLocaleDateString(undefined,{month:'short'})}</small></span><span className="plans-upcoming-row__copy"><strong>{plan.title}</strong><small>{plan.tasks.filter((task)=>task.done).length}/{plan.tasks.length} tasks · {plan.linked_entity_label || 'Workspace plan'}</small></span><ChevronRight size={14}/></button>) : <div className="plans-upcoming-empty"><CalendarDays size={20}/><p>No later plans in this window.</p><span>When something is coming up, it will show here.</span></div>}</aside>
    </div>

    <Modal open={editorOpen} title={editingPlan ? 'Edit plan' : `Plan for ${new Date(`${draft.plan_date}T12:00:00`).toLocaleDateString(undefined,{month:'short',day:'numeric'})}`} description="Make the intent clear, then split it into tasks small enough to finish." onClose={() => {if(!saving){setEditorOpen(false);setError('');}}} onSubmit={savePlan} submitLabel={editingPlan?'Save changes':'Create plan'} busy={saving} wide>
      <div className="plan-editor-grid">
        <Field label="Plan title"><input autoFocus required maxLength={180} value={draft.title} onChange={(event)=>updateDraft('title',event.target.value)} placeholder="e.g. Ship Gland emitter foundation"/></Field>
        <Field label="Date"><input required type="date" value={draft.plan_date} onChange={(event)=>updateDraft('plan_date',event.target.value)}/></Field>
        <Field label="Description / intention"><textarea rows={3} maxLength={10000} value={draft.description} onChange={(event)=>updateDraft('description',event.target.value)} placeholder="What would make this plan successful?"/></Field>
        <div className="plan-editor-link"><Field label="Connect this plan to"><select value={draft.linked_entity_type} onChange={(event)=>updateDraft('linked_entity_type',event.target.value)}><option value="">No linked entity</option><option value="issue">Issue</option><option value="repository">Repository</option></select></Field>
          {draft.linked_entity_type==='issue' && <Field label="Issue"><select required value={draft.linked_entity_id} onChange={(event)=>updateDraft('linked_entity_id',event.target.value)}><option value="">Choose an issue…</option>{issues.map((issue)=><option key={issue.id} value={issue.id}>{issue.repository_name}#{issue.number} · {issue.title}</option>)}</select></Field>}
          {draft.linked_entity_type==='repository' && <Field label="Repository"><select required value={draft.linked_entity_id} onChange={(event)=>updateDraft('linked_entity_id',event.target.value)}><option value="">Choose a repository…</option>{repositories.map((repo)=><option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></Field>}
        </div>
        {!editingPlan && <Field label="Task checklist" hint="One task per line. You can add or edit tasks after creating the plan."><textarea required rows={5} value={draft.tasks} onChange={(event)=>updateDraft('tasks',event.target.value)} placeholder={'Review current implementation\nWrite the first version\nTest and note what remains'}/></Field>}
        {editingPlan && <p className="plan-editor-hint">Existing checklist items stay as they are. Add more tasks by opening the plan after saving.</p>}
        {error && <div className="notice notice--error" role="alert">{error}</div>}
      </div>
    </Modal>
    <Modal open={Boolean(summaryPlans?.length)} title="Your plans for this day" description="Choose a plan to open its checklist, linked Issue or Repository, and notes." onClose={() => setSummaryPlans(null)} wide>
      <div className="plans-day-summary-list">
        {(summaryPlans ?? []).map((plan) => {
          const done = plan.tasks.filter((task) => task.done).length;
          const total = plan.tasks.length;
          return <button type="button" className="plans-day-summary-row" key={plan.id} onClick={() => { chooseDate(plan.plan_date); setSelectedPlan(plan); setSummaryPlans(null); }}>
            <span className={`plans-day-summary-row__icon ${total > 0 && done === total ? 'plans-day-summary-row__icon--done' : ''}`}><CalendarDays size={17}/></span>
            <span className="plans-day-summary-row__content"><strong>{plan.title}</strong><small>{plan.linked_entity_label || 'Workspace plan'}{plan.description ? ` · ${plan.description}` : ''}</small><span className="plans-day-summary-row__progress"><i><b style={{ width: `${total ? done / total * 100 : 0}%` }}/></i><em>{done}/{total} tasks</em></span></span>
            <ChevronRight size={16}/>
          </button>;
        })}
      </div>
    </Modal>
    <PlanDetailsModal plan={selectedPlan} open={Boolean(selectedPlan)} onClose={()=>setSelectedPlan(null)} onChanged={refreshAfterChange} onDeleted={deletedPlan} onEdit={startEdit}/>
  </div>;
}
