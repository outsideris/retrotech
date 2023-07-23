import Link from 'next/link'
import Image from 'next/image'

export default function Badges({ link }) {
    return (
        <div className="badges">
            <Link href={link}>
                <Image src="/badges/apple.svg" alt="Listen on Apple Podcasts" width={300} height={0} priority className="badge" />
            </Link>
            <Link href={link} className="youtube">
                <Image src="/badges/youtube.svg" alt="Available on YouTube" width={240} height={0} priority className="badge youtube" />
            </Link>
            <Link href={link}>
                <Image src="/badges/google.svg" alt="Listen on Google Podcasts" width={300} height={0} priority className="badge" />
            </Link>
            <Link href={link}>
                <Image src="/badges/spotify.svg" alt="Listen on Spotify" width={300} height={0} priority className="badge spotify" />
            </Link>
        </div>
    )
}
