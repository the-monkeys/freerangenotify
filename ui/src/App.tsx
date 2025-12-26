import React from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';
import LandingPage from './pages/LandingPage';
import AppsList from './pages/AppsList';
import AppDetail from './pages/AppDetail';
import Dashboard from './pages/Dashboard';
import Header from './components/Header';
import Footer from './components/Footer';
import './index.css';

const App: React.FC = () => {
  return (
    <Router>
      <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
        <Header />
        <main style={{ flex: 1 }}>
          <Routes>
            <Route path="/" element={<LandingPage />} />
            <Route path="/apps" element={<AppsList />} />
            <Route path="/apps/:id" element={<AppDetail />} />
            <Route path="/dashboard" element={<Dashboard />} />
          </Routes>
        </main>
        <Footer />
      </div>
    </Router>
  );
};

export default App;