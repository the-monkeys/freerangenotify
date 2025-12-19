import React from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import AppsList from './pages/AppsList';
import AppDetail from './pages/AppDetail';
import Notifications from './pages/Notifications';
import Templates from './pages/Templates';
import Header from './components/Header';
import './index.css';

const App: React.FC = () => {
  return (
    <Router>
      <Header />
      <Routes>
        <Route path="/" element={<AppsList />} />
        <Route path="/apps/:id" element={<AppDetail />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/templates" element={<Templates />} />
      </Routes>
    </Router>
  );
};

export default App;