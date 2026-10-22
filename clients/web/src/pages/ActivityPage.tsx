import { useCallback, useEffect, useMemo, useState } from 'react';
import { Activity, CalendarDays, Check, ChevronDown, ChevronUp, Flag, Filter, Flame, ListFilter, RotateCw, Search, Sparkles, X } from 'lucide-react';
import { api } from '@/lib/api';
import type { ActivityEvent, Plan } from '@/types/idios';
import { ActivityList } from '@/components/ActivityList';
import { Button, PageHeading } from '@/components/ui/primitives';

const HISTORY_LIMIT = 100;
const INITIAL_VISIBLE = 25;

type ActivityDay = { date: Date; key: string; count: number; level: number; plannedTotal: number; plannedDone: number };
type CalendarCell = ActivityDay | null;
type ActivityMonth = { key: string; label: string; shortLabel: string; year: number; isCurrent: boolean; eventCount: number; activeDays: number; cells: CalendarCell[] };

const ROUGH_DAYS_STORAGE_KEY = 'idios.activity.roughDays.v1';

export function ActivityPage() {
  const [items, setItems] = useState<ActivityEvent[]>([]);
  const [plans, setPlans] = useState<Plan[]>([]);
  const [visibleCount, setVisibleCount] = useState(INITIAL_VISIBLE);
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const [eventsExpanded, setEventsExpanded] = useState(false);
  const [selectedMonthKey, setSelectedMonthKey] = useState(() => monthKey(new Date()));
  const [roughDays, setRoughDays] = useState<Set<string>>(() => readRoughDays());
  const [query, setQuery] = useState('');
  const [entityType, setEntityType] = useState('all');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const load = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const today = new Date();
      const earliest = new Date(today.getFullYear(), today.getMonth() - 11, 1);
      const [events, datedPlans] = await Promise.all([
        api.activity({ limit: HISTORY_LIMIT }),
        api.plans({ from: dayKey(earliest) ?? undefined, to: dayKey(today) ?? undefined }),
      ]);
      setItems(events);
      setPlans(datedPlans);
    }
    catch (err) { setError(err instanceof Error ? err.message : 'Unable to load activity and plan progress.'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    try { localStorage.setItem(ROUGH_DAYS_STORAGE_KEY, JSON.stringify([...roughDays].sort())); }
    catch { /* Day markers remain usable for this session when storage is unavailable. */ }
  }, [roughDays]);

  const chooseDay = useCallback((key: string) => {
    const nextDay = key === selectedDay ? null : key;
    setSelectedDay(nextDay);
    // Opening a date from the calendar should reveal that date's actual events.
    setEventsExpanded(Boolean(nextDay) || Boolean(query.trim() || entityType !== 'all'));
  }, [selectedDay, query, entityType]);

  const toggleRoughDay = useCallback((key: string) => {
    setRoughDays((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }, []);

  const dayCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const item of items) {
      const key = dayKey(new Date(item.created_at));
      if (key) counts.set(key, (counts.get(key) ?? 0) + 1);
    }
    return counts;
  }, [items]);
  const planDayCounts = useMemo(() => {
    const counts = new Map<string, { total: number; done: number }>();
    for (const plan of plans) {
      const current = counts.get(plan.plan_date) ?? { total: 0, done: 0 };
      for (const task of plan.tasks ?? []) {
        current.total += 1;
        if (task.done) current.done += 1;
      }
      counts.set(plan.plan_date, current);
    }
    return counts;
  }, [plans]);

  const { calendarMonths, startDate, endDate, periodEventTotal, activeDayTotal, busiestMonth, maxMonthEvents } = useMemo(() => {
    // Build twelve real calendar-month cards rather than a GitHub-style strip.
    // The current month stops at today, so no future dates become selectable.
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const earliest = new Date(today.getFullYear(), today.getMonth() - 11, 1);
    const months: ActivityMonth[] = [];

    for (let offset = 0; offset < 12; offset += 1) {
      const monthStart = new Date(earliest.getFullYear(), earliest.getMonth() + offset, 1);
      const monthEnd = offset === 11 ? today : new Date(monthStart.getFullYear(), monthStart.getMonth() + 1, 0);
      // Every month uses the same 6 × 7 canvas, so switching months never
      // changes the calendar panel's height. Future dates remain empty slots.
      const cells: CalendarCell[] = Array.from({ length: 42 }, () => null);
      let eventCount = 0;
      let activeDays = 0;

      for (let dayOfMonth = 1; dayOfMonth <= monthEnd.getDate(); dayOfMonth += 1) {
        const date = new Date(monthStart.getFullYear(), monthStart.getMonth(), dayOfMonth);
        const key = dayKey(date) ?? '';
        const count = dayCounts.get(key) ?? 0;
        eventCount += count;
        if (count > 0) activeDays += 1;
        const planned = planDayCounts.get(key) ?? { total: 0, done: 0 };
        cells[monthStart.getDay() + dayOfMonth - 1] = { date, key, count, level: count === 0 ? 0 : count === 1 ? 1 : count <= 3 ? 2 : count <= 6 ? 3 : 4, plannedTotal: planned.total, plannedDone: planned.done };
      }

      months.push({
        key: monthKey(monthStart),
        label: monthStart.toLocaleDateString(undefined, { month: 'long', year: 'numeric' }),
        shortLabel: monthStart.toLocaleDateString(undefined, { month: 'short' }),
        year: monthStart.getFullYear(),
        isCurrent: monthStart.getFullYear() === today.getFullYear() && monthStart.getMonth() === today.getMonth(),
        eventCount,
        activeDays,
        cells,
      });
    }

    const periodEventTotal = months.reduce((total, month) => total + month.eventCount, 0);
    const activeDayTotal = months.reduce((total, month) => total + month.activeDays, 0);
    const busiestMonth = months.reduce<ActivityMonth | null>((best, month) => month.eventCount > (best?.eventCount ?? 0) ? month : best, null);
    const maxMonthEvents = Math.max(0, ...months.map((month) => month.eventCount));
    return { calendarMonths: months, startDate: earliest, endDate: today, periodEventTotal, activeDayTotal, busiestMonth, maxMonthEvents };
  }, [dayCounts, planDayCounts]);

  const selectedMonth = calendarMonths.find((month) => month.key === selectedMonthKey) ?? calendarMonths[calendarMonths.length - 1];
  const selectedMonthIndex = Math.max(0, calendarMonths.findIndex((month) => month.key === selectedMonth?.key));
  const selectedDayIsRough = selectedDay ? roughDays.has(selectedDay) : false;
  const selectedMonthWeeks = selectedMonth ? Array.from({ length: 6 }, (_, index) => {
    const weekDays = selectedMonth.cells.slice(index * 7, index * 7 + 7).filter((cell): cell is ActivityDay => cell !== null);
    return { label: `W${index + 1}`, events: weekDays.reduce((sum, day) => sum + day.count, 0), activeDays: weekDays.filter((day) => day.count > 0).length };
  }) : [];
  const maxWeekEvents = Math.max(1, ...selectedMonthWeeks.map((week) => week.events));
  const selectedMonthRoughCount = selectedMonth?.cells.filter((cell): cell is ActivityDay => Boolean(cell && roughDays.has(cell.key))).length ?? 0;
  const markedRoughDayCount = calendarMonths.reduce((total, month) => total + month.cells.filter((cell): cell is ActivityDay => Boolean(cell && roughDays.has(cell.key))).length, 0);

  const entityTypes = useMemo(() => [...new Set(items.map((item) => item.entity_type))].sort((a, b) => a.localeCompare(b)), [items]);
  const matchingItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter((item) => {
      const typeMatches = entityType === 'all' || item.entity_type === entityType;
      const text = `${item.entity_type} ${item.action} ${item.payload ?? ''}`.toLowerCase();
      const textMatches = !needle || text.includes(needle);
      const dayMatches = !selectedDay || dayKey(new Date(item.created_at)) === selectedDay;
      return typeMatches && textMatches && dayMatches;
    });
  }, [items, selectedDay, query, entityType]);
  const hasActiveFilters = Boolean(query.trim() || selectedDay || entityType !== 'all');
  const filteredItems = hasActiveFilters ? matchingItems : matchingItems.slice(0, visibleCount);
  const matchingTypeCount = new Set(matchingItems.map((item) => item.entity_type)).size;

  return <div className="page-stack activity-page">
    <PageHeading eyebrow="HISTORY" title="Activity" description="See when work happened at a glance, then open the events for details." actions={<Button variant="secondary" onClick={() => void load()} loading={loading}><RotateCw size={15} /> Refresh</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}

    <section className="activity-heatmap activity-atlas panel" aria-label="Activity calendar for the last twelve months, ending today">
      <div className="activity-atlas__heading">
        <div className="activity-atlas__title">
          <span className="activity-atlas__mark"><Activity size={18} /></span>
          <div>
            <span className="activity-atlas__eyebrow"><span className="activity-atlas__live-dot" /> ACTIVITY RHYTHM</span>
            <h2>Activity atlas</h2>
            <p>{startDate.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })} — {endDate.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}<span className="activity-atlas__separator">/</span> each date opens its events</p>
          </div>
        </div>
        <div className="activity-atlas__heading-meta">
          <span className="activity-atlas__window"><CalendarDays size={12} /> 12 MONTH WINDOW</span>
          <span className="activity-atlas__loaded">Latest {items.length} events loaded</span>
        </div>
        {selectedDay && <button type="button" className="activity-clear-filter activity-atlas__clear" onClick={() => setSelectedDay(null)}><X size={13} /> Clear day filter</button>}
      </div>

      <div className="activity-atlas__stats" aria-label="Activity summary">
        <div className="activity-atlas__stat activity-atlas__stat--events">
          <span className="activity-atlas__stat-icon"><Sparkles size={15} /></span>
          <div className="activity-atlas__stat-copy"><span>EVENTS IN VIEW</span><strong>{periodEventTotal}</strong><small>from loaded history</small></div>
          <div className="activity-atlas__sparkline" aria-hidden="true">{calendarMonths.map((month) => <i key={month.key} style={{ height: `${maxMonthEvents ? Math.max(12, (month.eventCount / maxMonthEvents) * 100) : 12}%` }} />)}</div>
        </div>
        <div className="activity-atlas__stat activity-atlas__stat--days">
          <span className="activity-atlas__stat-icon"><CalendarDays size={15} /></span>
          <div className="activity-atlas__stat-copy"><span>ACTIVE DAYS</span><strong>{activeDayTotal}</strong><small>days with at least one event</small></div>
        </div>
        <div className="activity-atlas__stat activity-atlas__stat--peak">
          <span className="activity-atlas__stat-icon"><Flame size={15} /></span>
          <div className="activity-atlas__stat-copy"><span>BUSIEST MONTH</span><strong>{busiestMonth?.eventCount ? busiestMonth.shortLabel : '—'}</strong><small>{busiestMonth?.eventCount ? `${busiestMonth.eventCount} recorded events` : 'No events in loaded history'}</small></div>
        </div>
      </div>

      <div className="activity-atlas__calendar-heading activity-atlas__calendar-heading--selector">
        <div><span className="activity-atlas__section-kicker">EXPLORE YOUR HISTORY</span><h3>Choose a month</h3><p>Months stay compact; only the month you open shows its full calendar.</p></div>
        <span className="activity-atlas__today-tag"><i /> TODAY <strong>{endDate.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}</strong></span>
      </div>

      <div className="activity-month-selector" role="group" aria-label="Choose a month">
        {calendarMonths.map((month, monthIndex) => <button
          key={month.key}
          type="button"
          className={`activity-month-selector__item ${month.key === selectedMonth?.key ? 'activity-month-selector__item--selected' : ''} ${month.isCurrent ? 'activity-month-selector__item--current' : ''}`}
          onClick={() => {
            setSelectedMonthKey(month.key);
            setSelectedDay((current) => current && current.startsWith(`${month.key}-`) ? current : null);
          }}
          aria-pressed={month.key === selectedMonth?.key}
          aria-label={`${month.label}: ${month.eventCount} events across ${month.activeDays} active days`}
        >
          <span className="activity-month-selector__index">{String(monthIndex + 1).padStart(2, '0')}</span>
          <span className="activity-month-selector__copy"><strong>{month.shortLabel}</strong><small>{month.year}</small><em>{month.activeDays} active {month.activeDays === 1 ? 'day' : 'days'}</em></span>
          <span className="activity-month-selector__meter" aria-hidden="true"><i style={{ width: `${maxMonthEvents ? (month.eventCount / maxMonthEvents) * 100 : 0}%` }} /></span>
          {month.isCurrent && <span className="activity-month-selector__now">NOW</span>}
        </button>)}
      </div>

      {selectedMonth && <section className="activity-month-calendar" aria-label={`${selectedMonth.label} activity calendar`}>
        <header className="activity-month-calendar__header">
          <div className="activity-month-calendar__month-title">
            <span className="activity-month-calendar__overline">MONTH {String(selectedMonthIndex + 1).padStart(2, '0')} <span>/</span> 12</span>
            <h3>{selectedMonth.label}</h3>
            <p>{selectedMonth.eventCount} recorded {selectedMonth.eventCount === 1 ? 'event' : 'events'} <span>·</span> {selectedMonth.activeDays} active {selectedMonth.activeDays === 1 ? 'day' : 'days'}{selectedMonth.isCurrent ? ' · through today' : ''}</p>
          </div>
          <div className="activity-month-calendar__selected-action">
            {selectedDay ? <>
              <div className="activity-month-calendar__selected-date"><CalendarDays size={14} /><span>{new Date(`${selectedDay}T12:00:00`).toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}</span></div>
              <button type="button" className={`activity-rough-toggle ${selectedDayIsRough ? 'activity-rough-toggle--restore' : ''}`} onClick={() => toggleRoughDay(selectedDay)}>
                {selectedDayIsRough ? <Check size={14} /> : <Flag size={14} />}
                {selectedDayIsRough ? 'Restore activity colors' : 'Mark as rough day'}
              </button>
            </> : <span className="activity-month-calendar__hint">Select a day to inspect or mark it</span>}
          </div>
        </header>

        <div className="activity-month-calendar__layout">
          <div className="activity-month-calendar__body">
            <div className="activity-month-calendar__weekdays" aria-hidden="true">{['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'].map((weekday) => <span key={weekday}>{weekday}</span>)}</div>
            <div className="activity-month-calendar__days">
              {selectedMonth.cells.map((day, cellIndex) => day ? <button
                type="button"
                key={day.key}
                className={`activity-month-calendar__day activity-heatmap__day--${day.level} ${day.plannedTotal ? 'activity-month-calendar__day--planned' : ''} ${day.key === dayKey(endDate) ? 'activity-month-calendar__day--today' : ''} ${selectedDay === day.key ? 'activity-month-calendar__day--selected' : ''} ${roughDays.has(day.key) ? 'activity-month-calendar__day--rough' : ''}`}
                style={day.plannedTotal && !roughDays.has(day.key) ? { backgroundColor: planCompletionColor(day.plannedDone, day.plannedTotal), borderColor: planCompletionColor(day.plannedDone, day.plannedTotal), color: '#14262b' } : undefined}
                onClick={() => chooseDay(day.key)}
                onContextMenu={(event) => { event.preventDefault(); setSelectedDay(day.key); setEventsExpanded(true); toggleRoughDay(day.key); }}
                title={`${day.date.toLocaleDateString()}: ${day.count} ${day.count === 1 ? 'event' : 'events'}${day.plannedTotal ? ` · ${day.plannedDone}/${day.plannedTotal} planned tasks complete` : ''}${roughDays.has(day.key) ? ' · marked as a rough day' : ''}. Right-click to toggle rough day.`}
                aria-label={`${day.date.toLocaleDateString()}, ${day.count} ${day.count === 1 ? 'event' : 'events'}${day.plannedTotal ? `, ${day.plannedDone} of ${day.plannedTotal} planned tasks done` : ''}${roughDays.has(day.key) ? ', marked as rough day' : ''}`}
                aria-pressed={selectedDay === day.key}
              ><span>{day.date.getDate()}</span>{roughDays.has(day.key) && <Flag size={10} aria-hidden="true" />}</button> : <span className="activity-month-calendar__blank" key={`${selectedMonth.key}-blank-${cellIndex}`} aria-hidden="true" />)}
            </div>
          </div>
          <aside className="activity-month-calendar__insights" aria-label="Selected month summary">
            <div className="activity-month-calendar__insights-heading"><span>MONTHLY SNAPSHOT</span><Sparkles size={14} /></div>
            <div className="activity-month-calendar__insight-metrics">
              <div><span>Events</span><strong>{selectedMonth.eventCount}</strong></div>
              <div><span>Active days</span><strong>{selectedMonth.activeDays}</strong></div>
              <div className="activity-month-calendar__insight-metric--rough"><span>Rough days</span><strong>{selectedMonthRoughCount}</strong></div>
            </div>
            <div className="activity-month-calendar__weekly-heading"><span>WEEKLY PULSE</span><span>EVENTS</span></div>
            <div className="activity-month-calendar__weekly-bars">{selectedMonthWeeks.map((week) => <div className="activity-month-calendar__week" key={week.label}>
              <span>{week.label}</span><div><i style={{ width: `${week.events / maxWeekEvents * 100}%` }} /></div><strong>{week.events}</strong>
            </div>)}</div>
            <p className="activity-month-calendar__insight-note"><Flag size={12} /> Mark rough days as your personal record of days that did not go well. The marker stays red even when activity is high.</p>
          </aside>
        </div>
        <footer className="activity-month-calendar__footer">
          <div className="activity-atlas__legend"><span>Quiet</span>{[0, 1, 2, 3, 4].map((level) => <span key={level} className={`activity-atlas__legend-cell activity-heatmap__day--${level}`} />)}<span>Busy</span><span className="activity-atlas__legend-note">Activity uses green intensity; plan days blend red → teal by completion. Manual rough days stay red.</span></div>
          <div className="activity-month-calendar__footnote"><span className="activity-month-calendar__rough-key"><i /><strong>{markedRoughDayCount}</strong> rough {markedRoughDayCount === 1 ? 'day' : 'days'} marked</span><span>Use the selected-day button or right-click · saved in this browser</span></div>
        </footer>
      </section>}
    </section>


    <div className="activity-filterbar panel" aria-label="Filter activity">
      <label className="inline-search"><Search size={15} /><input value={query} onChange={(event) => { const value = event.target.value; setQuery(value); if (value.trim()) setEventsExpanded(true); }} placeholder="Find an activity…" /></label>
      <label className="filter-select"><Filter size={14} /><select value={entityType} onChange={(event) => { const value = event.target.value; setEntityType(value); if (value !== 'all') setEventsExpanded(true); }} aria-label="Filter activity type"><option value="all">All activity types</option>{entityTypes.map((type) => <option key={type} value={type}>{type.replaceAll('_', ' ')}</option>)}</select></label>
      {(query || entityType !== 'all' || selectedDay) && <Button variant="ghost" onClick={() => { setQuery(''); setEntityType('all'); setSelectedDay(null); setEventsExpanded(false); }}>Clear filters</Button>}
    </div>

    <section className={`activity-feed panel ${eventsExpanded ? 'activity-feed--expanded' : 'activity-feed--collapsed'}`} aria-label="Workspace event timeline">
      <div className="activity-feed__toolbar">
        <div className="activity-feed__heading-copy"><span className="activity-feed__icon"><Activity size={16} /></span><div><h2>{selectedDay ? new Date(`${selectedDay}T12:00:00`).toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' }) : 'Recent events'}</h2><p>{loading ? 'Updating activity…' : selectedDay ? 'Events recorded on the selected date.' : hasActiveFilters ? 'Events match your current filters.' : 'The calendar above is the overview. Expand only when you need the full event details.'}</p></div></div>
        <div className="activity-feed__toolbar-side">
          <div className="activity-feed__summary">
            <div className="activity-feed__metric"><strong>{filteredItems.length}</strong><span>{hasActiveFilters ? 'matches' : 'visible'}</span></div>
            <div className="activity-feed__metric"><strong>{items.length}</strong><span>loaded</span></div>
            <div className="activity-feed__metric"><strong>{matchingTypeCount}</strong><span>{matchingTypeCount === 1 ? 'event type' : 'event types'}</span></div>
          </div>
          <Button variant={eventsExpanded ? 'primary' : 'secondary'} onClick={() => setEventsExpanded((expanded) => !expanded)} aria-expanded={eventsExpanded}>
            <ListFilter size={14} /> {eventsExpanded ? 'Hide events' : 'Show events'} {eventsExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </Button>
        </div>
      </div>
      {eventsExpanded && <div className="activity-feed__details">
        {loading && !items.length ? <div className="loading-state"><Activity size={17} className="spin" /> Loading activity…</div> : <ActivityList items={filteredItems} />}
        {!hasActiveFilters && visibleCount < matchingItems.length && <div className="activity-feed__footer"><span>Older events are shown only when requested.</span><Button variant="secondary" onClick={() => setVisibleCount((current) => Math.min(current + INITIAL_VISIBLE, matchingItems.length))}>Show older activity</Button></div>}
        {selectedDay && <div className="activity-feed__footer"><span>Showing one selected day.</span><Button variant="secondary" onClick={() => { setSelectedDay(null); setEventsExpanded(false); }}>Clear selected day</Button></div>}
      </div>}
    </section>
  </div>;
}

function planCompletionColor(done: number, total: number): string {
  const ratio = total > 0 ? Math.max(0, Math.min(1, done / total)) : 0;
  // A fully unfinished plan is coral-red; a fully completed day becomes teal-blue.
  const start = [224, 91, 103];
  const end = [75, 190, 204];
  const channels = start.map((value, index) => Math.round(value + (end[index] - value) * ratio));
  return `rgb(${channels[0]} ${channels[1]} ${channels[2]})`;
}

function monthKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
}

function readRoughDays(): Set<string> {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(ROUGH_DAYS_STORAGE_KEY) ?? '[]');
    if (!Array.isArray(value)) return new Set();
    return new Set(value.filter((entry): entry is string => typeof entry === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(entry)));
  } catch {
    return new Set();
  }
}

function dayKey(date: Date): string | null {
  if (!Number.isFinite(date.getTime())) return null;
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}
