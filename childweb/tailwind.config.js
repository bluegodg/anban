/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: "class",
  content: [
    "./首页/*.html",
    "./留言/*.html",
    "./提醒/*.html",
    "./家人/*.html",
    "./家人编辑页/*.html",
    "./设置/*.html",
    "./登录页/*.html",
  ],
  theme: {
    extend: {
      "colors": {
        "on-secondary-container": "#726155","on-tertiary-container": "#00463e","error": "#ba1a1a",
        "on-primary": "#ffffff","background": "#fbf9f8","inverse-primary": "#ffb59f",
        "tertiary-container": "#2fbcaa","surface-white": "#FFFFFF","on-primary-fixed": "#3a0a00",
        "tertiary-fixed": "#77f7e4","tertiary": "#006b5f","primary-fixed-dim": "#ffb59f",
        "surface-tint": "#9a4429","primary": "#9a4429","text-secondary": "#777777",
        "surface-container-high": "#eae8e7","inverse-surface": "#303030","on-secondary": "#ffffff",
        "on-surface-variant": "#55423d","warning": "#FFD88C","tertiary-fixed-dim": "#58dbc8",
        "on-background": "#1b1c1c","secondary": "#6c5b50","outline-variant": "#dcc1b9",
        "surface-bright": "#fbf9f8","surface-container-highest": "#e4e2e1","error-container": "#ffdad6",
        "on-primary-container": "#70250c","inverse-on-surface": "#f3f0f0","background-cream": "#FAF6F0",
        "success": "#6FCF97","danger": "#F28C82","on-tertiary": "#ffffff","on-surface": "#1b1c1c",
        "primary-container": "#f78c6b","secondary-fixed": "#f6decf","surface": "#fbf9f8",
        "on-primary-fixed-variant": "#7c2e14","secondary-container": "#f6decf","surface-container": "#f0eded",
        "on-error": "#ffffff","on-secondary-fixed": "#251910","on-secondary-fixed-variant": "#534439",
        "outline": "#88726c","secondary-fixed-dim": "#d9c2b4","surface-container-lowest": "#ffffff",
        "surface-dim": "#dcd9d9","gradient-primary-end": "#FF9B7A","primary-fixed": "#ffdbd0",
        "on-error-container": "#93000a","on-tertiary-fixed-variant": "#005047","on-tertiary-fixed": "#00201c",
        "surface-container-low": "#f6f3f2","surface-variant": "#e4e2e1","gradient-primary-start": "#F78C6B",
        "divider-warm": "#EFE8E1"
      },
      "borderRadius": {"DEFAULT": "0.25rem","lg": "0.5rem","xl": "0.75rem","full": "9999px"},
      "spacing": {"container-max-width": "430px","stack-default": "12px","card-gap": "16px","section-gap": "32px","stack-tight": "8px","page-margin": "20px"},
      "fontFamily": {"body-md": ["Be Vietnam Pro"],"label-md": ["Be Vietnam Pro"],"label-sm": ["Be Vietnam Pro"],"body-lg": ["Be Vietnam Pro"],"display-lg": ["Plus Jakarta Sans"],"display-lg-mobile": ["Plus Jakarta Sans"],"title-lg": ["Be Vietnam Pro"],"headline-md": ["Plus Jakarta Sans"]},
      "fontSize": {"body-md": ["15px",{"lineHeight":"22px","fontWeight":"400"}],"label-md": ["13px",{"lineHeight":"18px","letterSpacing":"0.01em","fontWeight":"500"}],"label-sm": ["11px",{"lineHeight":"16px","letterSpacing":"0.03em","fontWeight":"600"}],"body-lg": ["17px",{"lineHeight":"26px","fontWeight":"400"}],"display-lg": ["32px",{"lineHeight":"40px","letterSpacing":"-0.02em","fontWeight":"700"}],"display-lg-mobile": ["28px",{"lineHeight":"36px","fontWeight":"700"}],"title-lg": ["20px",{"lineHeight":"28px","fontWeight":"600"}],"headline-md": ["24px",{"lineHeight":"32px","fontWeight":"600"}]}
    },
  },
}