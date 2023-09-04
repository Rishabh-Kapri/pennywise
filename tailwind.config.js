/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ['./src/**/*.{html,ts}', './node_modules/flowbite/**/*.js'],
  theme: {
    extend: {
      zIndex: {
        100: '100',
      },
      borderWidth: {
        3: '3px',
      },
      flex: {
        33: '0 0 33.33333%',
        20: '0 0 20%',
        40: '0 0 40%',
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
      },
    },
  },
  plugins: [require('flowbite/plugin')],
};
