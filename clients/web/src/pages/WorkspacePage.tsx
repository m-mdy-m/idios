import { useEffect, useState } from 'react';
import { Bookmark, Check, CircleUserRound, ExternalLink, FolderGit2, GitPullRequest, Pin, Plus, UserRound, X } from 'lucide-react';
import { Link } from 'react-router';
import { api } from '@/lib/api';
import type { ProtocolCommand, WorkspacePin } from '@/types/idios';
import { Badge, Button, Card, EmptyState, PageHeading } from '@/components/ui/primitives';
import { CommandRunner } from '@/components/commands/CommandRunner';
import { useWorkspaceStore } from '@/store/workspace-store';

type Profile = { username: string; full_name: string; focus?: string; topics?: string; website?: string; bio: string; readme: string; updated_at: string };

export function WorkspacePage() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [pins, setPins] = useState<WorkspacePin[]>([]);
  const [commands, setCommands] = useState<ProtocolCommand[]>([]);
  const [selectedCommand, setSelectedCommand] = useState<ProtocolCommand | null>(null);
  const [initialValues, setInitialValues] = useState<Record<string, unknown>>({});
  const [error, setError] = useState('');
  const { load, dashboard } = useWorkspaceStore();

  async function refresh() {
    setError('');
    try {
      const [nextProfile, nextPins, catalog] = await Promise.all([api.profile(), api.pins(), api.protocol()]);
      setProfile(nextProfile); setPins(nextPins); setCommands(catalog.commands);
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to load workspace profile.'); }
  }
  useEffect(() => { void refresh(); }, []);
  function run(id: string, values: Record<string, unknown> = {}) {
    const command = commands.find((item) => item.id === id);
    if (!command) { setError(`Action ${id} is unavailable in the shared protocol.`); return; }
    setInitialValues(values); setSelectedCommand(command);
  }
  async function complete() { setSelectedCommand(null); await Promise.all([refresh(), load(true)]); }

  return <div className="page-stack">
    <PageHeading eyebrow="PERSONAL WORKSPACE" title="Profile & pins" description="Your workspace identity and pinned references live in the same local Engine as the TUI." actions={<Button variant="secondary" onClick={() => run('workspace.profile.update', profile ? { username: profile.username, full_name: profile.full_name, focus: profile.focus ?? '', topics: profile.topics ?? '', website: profile.website ?? '', bio: profile.bio, readme: profile.readme } : {})}><UserRound size={15} /> Edit profile</Button>} />
    {error && <div className="notice notice--error" role="alert">{error}</div>}
    <div className="workspace-profile-grid">
      <Card className="workspace-profile-card">
        <div className="workspace-profile-top"><span className="workspace-profile-avatar"><CircleUserRound size={25} /></span><Badge tone="green"><Check size={12} /> Local profile</Badge></div>
        <p className="eyebrow">WORKSPACE IDENTITY</p>
        <h2>{profile?.full_name || profile?.username || 'Personal workspace'}</h2>
        <p className="workspace-profile-username">{profile?.username ? `@${profile.username}` : 'Choose a username to make references easier to recognize.'}</p>
        <p className="workspace-profile-bio">{profile?.bio || 'Add a short bio about the work this workspace helps you do.'}</p>
        <div className="workspace-profile-details">
          <div><span>Current focus</span><strong>{profile?.focus || 'Not set yet'}</strong></div>
          <div><span>Topics & tools</span><strong>{profile?.topics || 'Not set yet'}</strong></div>
          {profile?.website && <a href={profile.website} target="_blank" rel="noreferrer"><ExternalLink size={13} /> {profile.website}</a>}
        </div>
        <div className="workspace-profile-stats"><div><strong>{dashboard?.repositories.length ?? 0}</strong><span>Repositories</span></div><div><strong>{dashboard?.issues.length ?? 0}</strong><span>Issues</span></div><div><strong>{dashboard?.projects.length ?? 0}</strong><span>Projects</span></div><div><strong>{dashboard?.notes.length ?? 0}</strong><span>Notes</span></div></div>
        {profile?.readme && <div className="workspace-readme"><p className="eyebrow">WORKSPACE README</p><p>{profile.readme}</p></div>}
        <div className="workspace-profile-footer"><span>Stored in this IDIOS workspace</span><span>{profile?.updated_at ? new Date(profile.updated_at).toLocaleDateString() : 'Not updated yet'}</span></div>
      </Card>
      <Card className="workspace-pins-card">
        <div className="section-heading"><div><p className="eyebrow">QUICK ACCESS</p><h2>Pinned references</h2></div><Button variant="secondary" onClick={() => run('workspace.pin.toggle')}><Plus size={15} /> Pin item</Button></div>
        {pins.length ? <div className="workspace-pin-list">{pins.map((pin) => {
          const type = pin.entity_type.toLowerCase().replaceAll('-', '_');
          const repository = dashboard?.repositories.find((item) => item.id === pin.entity_id);
          const issue = dashboard?.issues.find((item) => item.id === pin.entity_id);
          const project = dashboard?.projects.find((item) => item.id === pin.entity_id);
          const note = dashboard?.notes.find((item) => item.id === pin.entity_id);
          const rawLabel = pin.label?.trim() ?? '';
          const readableLabel = rawLabel && rawLabel !== pin.entity_id && !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(rawLabel) ? rawLabel : '';
          const title = readableLabel || repository?.name || (issue ? `${issue.repository_name}#${issue.number} · ${issue.title}` : '') || project?.name || note?.title || `${type.replaceAll('_', ' ')} reference`;
          const destination = repository ? `/repositories/${repository.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}` : issue ? `/issues?issue=${encodeURIComponent(issue.id)}` : project ? `/projects?project=${encodeURIComponent(project.id)}` : note ? '/notes' : type === 'pull_request' ? '/pull-requests' : type === 'concept' ? '/concepts' : type === 'label' || type === 'milestone' ? '/labels' : '/';
          const glyph = type === 'repository' ? <FolderGit2 size={15} /> : type === 'pull_request' ? <GitPullRequest size={15} /> : <Pin size={15} />;
          return <div className="workspace-pin-row" key={`${pin.entity_type}:${pin.entity_id}`}><span className="workspace-pin-mark" style={{ color: pin.color === 'accent' ? 'var(--accent)' : `var(--${pin.color}, var(--accent))` }}>{glyph}</span><Link className="workspace-pin-copy" to={destination}><strong>{title}</strong><span>{type.replaceAll('_', ' ')} · Open pinned item</span></Link><button className="icon-button" aria-label={`Unpin ${title}`} title="Unpin" onClick={() => run('workspace.pin.toggle', { entity_type: pin.entity_type, entity_ref: pin.entity_id, label: pin.label ?? '', color: pin.color })}><X size={14} /></button></div>;
        })}</div> : <EmptyState title="Nothing pinned yet" body="Pin an issue, repository, project, concept, note, label, or milestone for quick access." icon={<Bookmark size={21} />} />}
      </Card>
    </div>
    <Card className="workspace-profile-hint"><Pin size={16} /><p>Pin state is stored once in the Engine. The Web, TUI, and future clients see the same references after their next refresh.</p></Card>
    <CommandRunner command={selectedCommand} open={Boolean(selectedCommand)} initialValues={initialValues} onClose={() => setSelectedCommand(null)} onSuccess={complete} />
  </div>;
}
