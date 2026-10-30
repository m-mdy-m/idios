import { useEffect, useState } from 'react';
import { ArrowRight, BookOpen, CheckSquare, Compass, FolderKanban, GitBranch, Layers3, Sparkles } from 'lucide-react';
import { Link, useNavigate } from 'react-router';
import { api } from '@/lib/api';
import { useWorkspaceStore } from '@/store/workspace-store';
import type { FeatureDefinition } from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/design-system';

const iconByID: Record<string, typeof Compass> = {
  repositories: Layers3,
  issues: CheckSquare,
  projects: FolderKanban,
  graph: GitBranch,
  notes: BookOpen,
  commands: Sparkles,
};

export function IntroPage() {
  const { dashboard, load } = useWorkspaceStore();
  const navigate = useNavigate();
  const [features, setFeatures] = useState<FeatureDefinition[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    void api.features().then((payload) => {
      if (active) setFeatures(payload.features.filter((item) => item.web_navigation && item.id !== 'overview'));
    }).catch((err: unknown) => {
      if (active) setError(err instanceof Error ? err.message : 'Unable to load the shared feature catalog.');
    });
    return () => { active = false; };
  }, []);

  useEffect(() => { if (!dashboard) void load(); }, [dashboard, load]);
  const hasRepository = (dashboard?.repositories.length ?? 0) > 0;
  const hasIssue = (dashboard?.issues.length ?? 0) > 0;

  return <div className="page-stack intro-page">
    <PageHeading eyebrow="GETTING STARTED" title="Your workspace, one clear path." description="IDIOS keeps projects, issues, knowledge and reminders in one local Engine. The Web workspace and CLI use the same data and workflow rules." actions={<Badge tone="green"><Compass size={13} /> Local-first</Badge>} />
    <Card className="intro-hero-card">
      <div className="intro-hero-card__copy"><p className="eyebrow">START WHERE YOU ARE</p><h2>Build a workspace that remembers the context.</h2><p>Create a repository, capture the next unit of work, then connect concepts and notes as you go. The Go Engine owns the rules; every client shows the same result.</p><div className="intro-hero-actions"><Button onClick={() => navigate(hasRepository ? '/issues' : '/repositories')}>{hasRepository ? 'Go to issues' : 'Create a repository'} <ArrowRight size={15} /></Button><Button variant="secondary" onClick={() => navigate('/commands')}>Explore actions <Sparkles size={15} /></Button></div></div>
      <div className="intro-progress"><div><span>Workspace setup</span><strong>{Number(hasRepository) + Number(hasIssue)}/2</strong></div><div className="progress-track"><span style={{ width: `${(Number(hasRepository) + Number(hasIssue)) * 50}%` }} /></div><div className="intro-check-row"><span className={hasRepository ? 'intro-check intro-check--done' : 'intro-check'}>{hasRepository ? '✓' : '1'}</span><div><strong>Create a repository</strong><small>{hasRepository ? `${dashboard?.repositories.length} workspace(s) ready` : 'A home for issues and milestones'}</small></div></div><div className="intro-check-row"><span className={hasIssue ? 'intro-check intro-check--done' : 'intro-check'}>{hasIssue ? '✓' : '2'}</span><div><strong>Track the first issue</strong><small>{hasIssue ? `${dashboard?.issues.length} issue(s) in your Engine` : 'Make the next action explicit'}</small></div></div></div>
    </Card>
    <div className="section-heading"><div><p className="eyebrow">WORKSPACE MAP</p><h2>Everything stays connected</h2></div><span className="table-secondary">{features.length} shared destinations</span></div>
    {error && <div className="notice notice--warning" role="alert">{error}</div>}
    {features.length ? <div className="intro-feature-grid">{features.map((feature) => { const Icon = iconByID[feature.id] ?? Sparkles; return <Link key={feature.id} to={feature.web_path || '/'} className="intro-feature-card"><span className="intro-feature-card__icon"><Icon size={18} /></span><div><h3>{feature.label}</h3><p>{feature.description}</p></div><ArrowRight size={15} /></Link>; })}</div> : <Card><EmptyState title="Feature catalog is loading" body="The local Go server provides the list of supported destinations to both clients." icon={<Compass size={20} />} /></Card>}
    <Card className="intro-protocol-note"><div className="intro-protocol-note__icon"><Sparkles size={18} /></div><div><h3>One command contract</h3><p>Use the Action center for supported operations. A new protocol command becomes available through the Web API and CLI without duplicating validation or business logic.</p><Link to="/commands">Open Action center <ArrowRight size={14} /></Link></div></Card>
  </div>;
}
