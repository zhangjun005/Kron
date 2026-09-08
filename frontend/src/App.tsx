import { Router, Route } from '@solidjs/router';
import { Home } from './routes/Home';
import { Project } from './routes/Project';
import { Tabs } from './routes/Tabs';

export function App() {
  return (
    <Router>
      <Route path="/" component={Home} />
      <Route path="/project" component={Project} />
      <Route path="/project/:projectId/*" component={Tabs} />
    </Router>
  );
}
