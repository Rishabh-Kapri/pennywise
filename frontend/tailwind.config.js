/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,ts}', './node_modules/flowbite/**/*.js'],
  darkMode: 'class',
  theme: {
    extend: {
      zIndex: {
        100: '100',
        999: 999,
      },
      borderWidth: {
        3: '3px',
      },
      borderRadius: {
        'small': '0.2rem', 
      },
      flex: {
        5: '0 0 5%',
        12.5: '0 0 12.5%',
        20: '0 0 20%',
        33: '0 0 33.33333%',
        40: '0 0 40%',
      },
      width: {
        21: '5.25rem',
      },
      backgroundColor: {
        'primary-blue': '#2563eb',
        'primary-blue-hover': '#1d4ed8',
        'primary-dark': '#1f2937',
        'primary-dark-hover': '#374151',
        'primary-red': '#dc2626',
        'primary-red-hover': '#b91c1c',
        'sidebar-dark': '#111827',
        'sidebar-dark-hover': '#1f2937',
        'light-grey': '#f3f4f6',
        'dark-grey': '#1f2937',
        'budget-green': '#6d9f38',
        'budget-green-hover': '#81b83d',
      },
      colors: {
        zinc: {
          850: '#232325',
        },
        neutral: {
          850: '#2c2c2e',
        },
        'budget-green': '#6d9f38',
        'budget-green-hover': '#81b83d',
        'pw-bg-light': '#F4F6F6',
        'pw-text-primary-light': '#2C383F',
        'pw-text-secondary-light': '#37474F',
        'pw-surface-light': '#FFFFFF',
        'pw-bg-dark': '#2C383F',
        'pw-text-primary-dark': '#F4F6F6',
        'pw-text-secondary-dark': '#D9E7E5',
        'pw-surface-dark': '#37474F',
        'pw-card-dark-teal': '#42887C',
        'pw-card-muted-blue': '#81B2CA',
        'pw-accent-yellow': '#FAB512',
        'pw-text-on-dark': '#FFFFFF',
        'pw-icon-bg-green': '#D9E7E5',
        'pw-icon-green': '#42887C',
        'pw-icon-bg-purple': '#E6E2E6',
        'pw-icon-purple': '#836F81',
      },
      textColor: {
        'budget-green': '#6d9f38',
        'budget-green-hover': '#81b83d',
      },
    },
  },
  plugins: [require('flowbite/plugin')],
};
