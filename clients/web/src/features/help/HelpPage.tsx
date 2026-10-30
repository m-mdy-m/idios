import { ArrowRight, Command, GitBranch, Keyboard, Layers3, ShieldCheck, Sparkles } from 'lucide-react';
import { Link } from 'react-router';
import { Card, PageHeading } from '@/design-system';

const shortcuts = [
  { keys: ['Ctrl', 'K'], label: 'Search the workspace', detail: 'Find issues, repositories, projects and notes from any page.' },
  { keys: ['Ctrl', 'Shift', 'P'], label: 'Open the Action center', detail: 'Run any action published in the shared Go protocol catalog.' },
  { keys: ['Esc'], label: 'Close the current overlay', detail: 'Dismiss search, quick-create, or a modal without changing pages.' },
];

const principles = [
  { icon: Layers3, title: 'One Engine', body: 'Workflow validation, relationship rules, persistence and transactions stay in the Go Application layer. Web and CLI are clients of that layer.' },
  { icon: GitBranch, title: 'One protocol', body: 'The command catalog defines action IDs, fields, constraints and dynamic choices. The Web Action center and CLI read the same catalog.' },
  { icon: ShieldCheck, title: 'Local by design', body: 'The server binds to Loopback by default. Workspace records are stored in your local database; the Web UI does not need a separate Node.js runtime.' },
];

export function HelpPage() {
  return <div className="page-stack">
    <PageHeading eyebrow="IDIOS HANDBOOK" title="Help & shortcuts" description="A quick guide to moving around the workspace and understanding how its clients fit together." />
    <div className="help-hero"><span className="help-hero__mark"><Sparkles size={24} /></span><div><p className="eyebrow">WORKSPACE, NOT ANOTHER SOURCE OF TRUTH</p><h2>One place to think, plan and track.</h2><p>Use the Web workspace for visual overviews and the CLI for terminal-first work. Both use the same Go Engine, configuration, database, scheduled work and action rules.</p></div></div>
    <section className="help-section"><div className="section-heading"><div><p className="eyebrow">KEYBOARD</p><h2>Shortcuts</h2></div><Keyboard size={18} /></div><div className="help-shortcut-list">{shortcuts.map((item) => <div className="help-shortcut" key={item.label}><div className="help-shortcut__keys">{item.keys.map((key) => <kbd key={key}>{key}</kbd>)}</div><div><strong>{item.label}</strong><p>{item.detail}</p></div></div>)}</div></section>
    <section className="help-section"><div className="section-heading"><div><p className="eyebrow">ARCHITECTURE</p><h2>How IDIOS stays in sync</h2></div><Command size={18} /></div><div className="help-principles">{principles.map(({ icon: Icon, title, body }) => <Card className="help-principle" key={title}><span><Icon size={18} /></span><h3>{title}</h3><p>{body}</p></Card>)}</div></section>
    <section className="help-next-actions"><div><p className="eyebrow">NEED TO DO SOMETHING?</p><h2>Start with an action.</h2><p>Commands and their input forms come from the same catalog used by the terminal command palette. When a new action is added to that Go catalog, it becomes available here automatically.</p></div><div className="help-next-actions__links"><Link to="/commands">Open Action center <ArrowRight size={15} /></Link><Link to="/settings">Configure IDIOS <ArrowRight size={15} /></Link><Link to="/logs">View diagnostics <ArrowRight size={15} /></Link></div></section>
  </div>;
}
