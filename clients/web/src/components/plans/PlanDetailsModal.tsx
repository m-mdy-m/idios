import { useEffect, useState } from 'react';
import { CalendarDays, CheckCircle2, Circle, ExternalLink, Link2, ListTodo, Plus, Trash2, Pencil } from 'lucide-react';
import { Link } from 'react-router';
import { api } from '@/lib/api';
import type { Plan } from '@/types/idios';
import { Badge, Button } from '@/components/ui/primitives';
import { Modal } from '@/components/ui/Modal';
import { stopPlanReminderSequence } from '@/lib/notification-sound';

export function PlanDetailsModal({ plan, open, onClose, onChanged, onDeleted, onEdit }: {
  plan: Plan | null;
  open: boolean;
  onClose: () => void;
  onChanged: () => void | Promise<void>;
  onDeleted?: (id: string) => void;
  onEdit?: (plan: Plan) => void;
}) {
  const [current, setCurrent] = useState<Plan | null>(plan);
  const [taskTitle, setTaskTitle] = useState('');
  const [busyTask, setBusyTask] = useState('');
  const [busyAdd, setBusyAdd] = useState(false);
  const [busyDelete, setBusyDelete] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => { setCurrent(plan); setError(''); setTaskTitle(''); if (open) stopPlanReminderSequence(); }, [plan?.id, plan, open]);

  if (!current) return <Modal open={false} title="Plan" onClose={onClose}><></></Modal>;
  const tasks = current.tasks ?? [];
  const completed = tasks.filter((task) => task.done).length;
  const progress = tasks.length ? Math.round(completed / tasks.length * 100) : 0;
  const relatedPath = current.linked_entity_type === 'issue'
    ? `/issues?issue=${encodeURIComponent(current.linked_entity_id ?? '')}`
    : current.linked_entity_type === 'repository'
      ? `/repositories/${encodeURIComponent(current.linked_entity_id ?? '')}`
      : '';

  async function reloadPlan() {
    const items = await api.plans({ from: current!.plan_date, to: current!.plan_date });
    const next = items.find((item) => item.id === current!.id);
    if (next) setCurrent(next);
    await onChanged();
  }
  async function toggleTask(id: string, done: boolean) {
    if (!current || busyTask) return;
    setBusyTask(id); setError('');
    try {
      const result = await api.setPlanTaskDone(current.id, id, done);
      if (done) stopPlanReminderSequence();
      setCurrent((value) => value ? { ...value, tasks: value.tasks.map((task) => task.id === id ? result : task) } : value);
      await onChanged();
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to update this task.'); }
    finally { setBusyTask(''); }
  }
  async function addTask() {
    if (!current || !taskTitle.trim() || busyAdd) return;
    setBusyAdd(true); setError('');
    try { await api.addPlanTask(current.id, taskTitle.trim()); setTaskTitle(''); await reloadPlan(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to add this task.'); }
    finally { setBusyAdd(false); }
  }
  async function deletePlan() {
    if (!current || busyDelete || !window.confirm(`Delete plan “${current.title}” and its checklist?`)) return;
    setBusyDelete(true); setError('');
    try { await api.deletePlan(current.id); onDeleted?.(current.id); onClose(); await onChanged(); }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to delete this plan.'); }
    finally { setBusyDelete(false); }
  }

  return <Modal open={open} title={current.title} description="Your dated plan and its task checklist." onClose={() => { stopPlanReminderSequence(); onClose(); }} wide>
    <div className="plan-detail-modal">
      <div className="plan-detail-meta">
        <span className="plan-date-chip"><CalendarDays size={14}/>{new Date(`${current.plan_date}T12:00:00`).toLocaleDateString(undefined, { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' })}</span>
        {current.linked_entity_type && current.linked_entity_label && relatedPath && <Link className="plan-related-link" to={relatedPath} onClick={() => { stopPlanReminderSequence(); onClose(); }}><Link2 size={14}/><span><small>LINKED TO {current.linked_entity_type.toUpperCase()}</small><strong>{current.linked_entity_label}</strong></span><ExternalLink size={13}/></Link>}
      </div>
      {current.description && <p className="plan-detail-description">{current.description}</p>}
      <div className="plan-checklist-heading"><div><span className="plan-section-icon"><ListTodo size={16}/></span><div><h3>Plan checklist</h3><p>Check off each step as you complete it.</p></div></div><Badge tone={progress === 100 && tasks.length ? 'green' : progress ? 'blue' : 'neutral'}>{completed}/{tasks.length} done</Badge></div>
      <div className="plan-progress-track"><i style={{ width: `${progress}%` }}/></div>
      <div className="plan-task-list">{tasks.length ? tasks.map((task) => <label className={`plan-task-row ${task.done ? 'plan-task-row--done' : ''}`} key={task.id}>
        <input type="checkbox" checked={task.done} disabled={busyTask === task.id} onChange={(event) => void toggleTask(task.id, event.target.checked)}/>
        <span className="plan-task-row__check" aria-hidden="true">{task.done ? <CheckCircle2 size={18}/> : <Circle size={18}/>}</span>
        <span className="plan-task-row__title">{task.title}</span>
        {busyTask === task.id && <span className="plan-task-row__busy">Saving…</span>}
      </label>) : <div className="plan-no-tasks"><ListTodo size={18}/><span>No tasks in this plan yet. Add the first next step below.</span></div>}</div>
      <div className="plan-add-task"><input value={taskTitle} onChange={(event) => setTaskTitle(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void addTask(); } }} placeholder="Add a small actionable task…" maxLength={500} aria-label="New task title"/><Button type="button" variant="secondary" disabled={!taskTitle.trim()} loading={busyAdd} onClick={() => void addTask()}><Plus size={14}/> Add task</Button></div>
      {tasks.length > 0 && <p className="plan-progress-note">{progress === 100 ? 'Everything on this plan is done. Take the win.' : `${tasks.length - completed} task${tasks.length - completed === 1 ? '' : 's'} left. One step at a time.`}</p>}
      {error && <div className="notice notice--error" role="alert">{error}</div>}
      <div className="plan-detail-footer"><span>Updated {new Date(current.updated_at).toLocaleString()}</span><div className="plan-detail-footer__actions">{onEdit && <Button type="button" variant="secondary" onClick={() => onEdit(current)}><Pencil size={14}/> Edit plan</Button>}<Button type="button" variant="danger" onClick={() => void deletePlan()} loading={busyDelete}><Trash2 size={14}/> Delete plan</Button></div></div>
    </div>
  </Modal>;
}
