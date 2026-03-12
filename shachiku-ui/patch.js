const fs = require('fs');
let code = fs.readFileSync('src/app/page.tsx', 'utf8');

const pollCode = `
  useEffect(() => {
    const interval = setInterval(() => {
      if (loading) return; // Do not fetch background memory while actively chatting
      fetch(\`\${API_URL}/memory\`)
        .then((res) => res.json())
        .then((data) => {
          if (Array.isArray(data)) {
            setMessages(prev => {
              if (prev.length === data.length) return prev; // Avoid unnecessary re-renders
              return data;
            });
          }
        })
        .catch(console.error);
    }, 3000);
    return () => clearInterval(interval);
  }, [API_URL, loading]);
`;

code = code.replace("}, [API_URL])", "}, [API_URL])" + pollCode);

fs.writeFileSync('src/app/page.tsx', code);
