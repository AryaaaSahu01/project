import {Link} from 'react-router'
import { APP_CONFIG } from '../config/app'
import '@fontsource/cormorant-garamond/latin-600.css'
import '../styles/identity.css'

export default function Brand() {
    return (
        <Link
            className='brand journal-brand'
            to='/today'
            aria-label={`${APP_CONFIG.name} home`}
        >
            <span className='brand-name'>{APP_CONFIG.name}</span>

            <svg 
                className='brand-pen'
                viewBox='0 0 32 44'
                fill='none'
                stroke='currentColor'
                strokeWidth='1.4'
                strokeLinecap='round'
                strokeLinejoin='round'
                aria-hidden='true'
                focusable='false'
            >
                <path d='M16 3 27 20 23 32H9L5 20 16 3Z'/>
                <path d='M16 3v14M9 32h14M10 36h12'/>
                <circle cx='16' cy='19' r='2'/>
                <path d= 'M5 41c7-3 15 2 23-1'/>
            </svg>
        </Link>
    )   
}