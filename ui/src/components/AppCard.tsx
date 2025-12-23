import React from 'react';

interface AppCardProps {
    title: string;
    description: string;
    onClick: () => void;
}

const AppCard: React.FC<AppCardProps> = ({ title, description, onClick }) => {
    return (
        <div className="app-card" onClick={onClick}>
            <h3>{title}</h3>
            <p>{description}</p>
        </div>
    );
};

export default AppCard;