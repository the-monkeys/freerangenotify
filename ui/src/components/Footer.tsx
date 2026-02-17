import React from 'react';

const Footer: React.FC = () => {
    const currentYear = new Date().getFullYear();
    return (
        <footer className="bg-gray-100 border-t border-gray-200 py-8 text-gray-500 text-sm mt-auto">
            <div className="mx-auto px-8">
                <div className="flex flex-col gap-1 items-center text-center">
                    <p className="text-gray-900 font-semibold text-sm mb-1">A product of BUDDHICINTAKA PVT. LTD.</p>
                    <p className="m-0 text-xs text-gray-400">&copy; {currentYear} BUDDHICINTAKA PVT. LTD. All rights reserved.</p>
                </div>
            </div>
        </footer>
    );
};

export default Footer;
